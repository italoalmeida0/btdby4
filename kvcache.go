package btdby4

// KvCacheProvider is a high-performance simulation of a real LLM
// prefix/KV cache, designed to run inside the WASM gateway build.
//
// Input:  request payload + protocol + namespace string.
// The namespace isolates entries the way real providers do, e.g.
// "provider|model|api-key" assembled by the gateway and passed in
// ready to use (it is used directly as the map key).
//
// Output: how many tokens hit the cache, how many were fresh, and
// how many were newly written — so the gateway can bill/estimate
// cached input even when the provider does not report it.
//
// Internals:
//   - one prefix trie (tree) per namespace, so many conversations /
//     branches can coexist under the same key;
//   - one node per content block/part in canonical order
//     (system/instructions, tools in order, then messages/items in
//     order, each expanded block-by-block), mimicking real KV order;
//   - first hash mismatch ends the prefix, the rest is fresh —
//     exactly like a real prefix cache invalidation;
//   - block identity is FNV-1a 64 (fast, no alloc), nodes store only
//     {hash, tokens}, never raw text;
//   - fixed TTL of 10 minutes, sliding (a hit refreshes the path);
//   - global memory cap (default 400MB) with automatic LRU
//     eviction; expiration is lazy + opportunistic sweep.

import (
	"bytes"
	"encoding/base64"
	json "github.com/goccy/go-json"
	"errors"
	"image"
	"strings"
	"sync"
	"time"

	"github.com/italoalmeida0/btdby4/codec"
)

// Management limits. Defaults: TTL 10min, 400MB. Override once via
// KvInit(ttlSeconds, maxMB) — e.g. KvInit(600, 400).
const (
	kvDefaultTTLMillis = int64(10 * 60 * 1000)
	kvDefaultMaxBytes  = int64(400 * 1024 * 1024)
	// kvTargetRatio is the fill level eviction aims for after overflow.
	kvTargetRatio = 0.95
	// kvNodeBytes approximates one trie node cost:
	// hash(8) + tokens(8) + lastAccess(8) + map entry/child overhead.
	kvNodeBytes = int64(128)
	// kvSweepEvery triggers an opportunistic expired+overflow sweep.
	kvSweepEvery = 128
)

// KvBlock is one canonical position of the prefix: the FNV-1a hash
// identifying the block content plus its content tokens (no safety
// margin — margin is folded back proportionally at the end so that
// total == cached + fresh always holds).
type KvBlock struct {
	Hash   uint64
	Tokens int
}

// KvResult is the per-request simulation outcome.
type KvResult struct {
	Total        int       `json:"total"`
	Cached       int       `json:"cached"`
	Fresh        int       `json:"fresh"`
	Written      int       `json:"written"`
	Hit          bool      `json:"hit"`
	HitRatio     float64   `json:"hit_ratio"`
	PrefixBlocks int       `json:"prefix_blocks"`
	TotalBlocks  int       `json:"total_blocks"`
	Breakdown    Breakdown `json:"breakdown"`
}

// KvStats is a global introspection snapshot for the gateway.
type KvStats struct {
	Namespaces int   `json:"namespaces"`
	Nodes      int64 `json:"nodes"`
	Branches   int64 `json:"branches"`
	Tokens     int64 `json:"tokens"`
	Bytes      int64 `json:"bytes"`
	MaxBytes   int64 `json:"max_bytes"`
	// AvailableBytes is MaxBytes - Bytes (clamped at zero): how much
	// cache memory is still free before LRU eviction kicks in.
	AvailableBytes int64 `json:"available_bytes"`
	// TTLSeconds is the configured sliding TTL (KvInit, default 600).
	TTLSeconds int64 `json:"ttl_seconds"`
}

// KvConfig reports the effective tunables (for kvInit echo / stats).
type KvConfig struct {
	TTLSeconds       int64 `json:"ttl_seconds"`
	MaxMB            int64 `json:"max_mb"`
	SeparateProtocol bool  `json:"separate_protocol"`
}

// KvConfigSnapshot returns the effective config.
func KvConfigSnapshot() KvConfig {
	kvMu.Lock()
	defer kvMu.Unlock()
	return KvConfig{
		TTLSeconds:       kvTTLMillis / 1000,
		MaxMB:            kvMaxBytes / (1024 * 1024),
		SeparateProtocol: kvSeparateProtocol,
	}
}

// kvNode is one trie node. The root has Hash 0 and Tokens 0.
type kvNode struct {
	hash       uint64
	tokens     int
	lastAccess int64
	children   map[uint64]*kvNode
}

// kvNamespace is one prefix tree.
type kvNamespace struct {
	root       *kvNode
	lastAccess int64
	nodeCount  int64
	tokenSum   int64
	byteSum    int64
}

var kvMu sync.Mutex
var kvStore = make(map[string]*kvNamespace)
var kvTotalNodes int64
var kvTotalTokens int64
var kvTotalBytes int64
var kvCalls int64

// Tunables set by KvInit (defaults above). Zero/negative keeps default.
var kvTTLMillis = kvDefaultTTLMillis
var kvMaxBytes = kvDefaultMaxBytes

// kvSeparateProtocol controls keying: true (default) keys by
// protocol + "\x00" + namespace; false keys by namespace only.
var kvSeparateProtocol = true

// KvInit configures the cache: ttlSeconds (default 600 = 10min) and
// maxMB (default 400). Call once at startup; zero/negative keeps the
// default for that field. It also clears existing entries, since TTL
// and size class are global.
func KvInit(ttlSeconds int64, maxMB int64) {
	kvMu.Lock()
	defer kvMu.Unlock()
	if ttlSeconds > 0 {
		kvTTLMillis = ttlSeconds * 1000
	} else {
		kvTTLMillis = kvDefaultTTLMillis
	}
	if maxMB > 0 {
		kvMaxBytes = maxMB * 1024 * 1024
	} else {
		kvMaxBytes = kvDefaultMaxBytes
	}
	kvStore = make(map[string]*kvNamespace)
	kvTotalNodes, kvTotalTokens, kvTotalBytes = 0, 0, 0
}

// KvSetSeparateProtocol toggles protocol-scoped keying. Default true:
// protocol + namespace form the key. When false, only namespace keys
// the entries (same hash in different protocols shares the prefix).
func KvSetSeparateProtocol(separate bool) {
	kvMu.Lock()
	defer kvMu.Unlock()
	kvSeparateProtocol = separate
}

// kvKey builds the store key honoring kvSeparateProtocol.
func kvKey(protocol, namespace string) string {
	if namespace == "" {
		namespace = "default"
	}
	if kvSeparateProtocol {
		return protocol + "\x00" + namespace
	}
	return namespace
}

// countChatToolFast counts a tool given its pre-marshaled params string
// (avoids marshaling twice when the KV path already needs the string
// for hashing).
func countChatToolFast(t ChatTool, pStr string) (int, error) {
	total := 0
	n, err := codec.FastCountNoErr(t.Function.Name)
	if err != nil {
		return 0, err
	}
	total += n
	if t.Function.Description != "" {
		n, err := codec.FastCountNoErr(t.Function.Description)
		if err != nil {
			return 0, err
		}
		total += n
	}
	if t.Function.Parameters != nil {
		n, err := codec.FastCountNoErr(pStr)
		if err != nil {
			return 0, err
		}
		total += n
	}
	return total, nil
}

// kvCountChatRequest counts + linearizes a decoded chat request in ONE walk.
// Shared by CountChatRequest (drops blocks) and KvLookup (keeps both).
// Returns (breakdown, blocks, err). ByMessage/ByTool are filled, so the
// Breakdown is identical to CountChatRequest.
func kvCountChatRequest(req ChatRequest, opts Options) (Breakdown, []KvBlock, error) {
	var out Breakdown
	var blocks []KvBlock
	if req.System != nil {
		blocks = kvAppendTextValue(blocks, "sys", "", req.System, opts)
		sum := 0
		for _, b := range blocks {
			sum += b.Tokens
		}
		out.System = sum
	}
	out.ByMessage = make([]int, len(req.Messages))
	msgBase := len(blocks)
	for i, m := range req.Messages {
		before := len(blocks)
		blocks = kvAppendChatMessageCount(blocks, m, opts, &out)
		n := 0
		for _, b := range blocks[before:] {
			n += b.Tokens
		}
		out.ByMessage[i] = n
		out.Messages += n
		_ = msgBase
	}
	nTools := 0
	for _, t := range req.Tools {
		h := kvHashNew()
		h = kvHashStr(h, "tool")
		h = kvHashSep(h)
		h = kvHashStr(h, t.Function.Name)
		h = kvHashSep(h)
		h = kvHashStr(h, t.Function.Description)
		h = kvHashSep(h)
		pStr := jsonString(t.Function.Parameters)
		h = kvHashStr(h, pStr)
		n, err := countChatToolFast(t, pStr)
		if err != nil {
			return out, nil, err
		}
		h = kvHashInt(h, n)
		blocks = append(blocks, KvBlock{Hash: h, Tokens: n})
		out.ByTool = append(out.ByTool, n)
		nTools += n
	}
	out.Tools = nTools
	out.TextTokens = out.System + out.Messages + out.Tools - out.Images
	out.Total = out.System + out.Messages + out.Tools
	applySafetyMargin(&out, opts, len(req.Messages), len(req.Tools), out.ImageCount)
	return out, blocks, nil
}

// KvLookup is the atomic check+update: it walks the namespace trie
// with the linearized blocks, reports cached/fresh, then commits the
// miss suffix as a new branch so the next request can hit it.
// Old branches stay alive until TTL/eviction — N conversations can
// coexist under one namespace.
func KvLookup(raw []byte, protocol, namespace string, opts Options) (KvResult, error) {
	var out KvResult
	blocks, bd, err := kvLinearize(raw, protocol, opts)
	if err != nil {
		return out, err
	}
	out.Breakdown = bd
	out.Total = bd.Total
	out.TotalBlocks = len(blocks)

	contentTotal := 0
	for _, b := range blocks {
		contentTotal += b.Tokens
	}

	now := time.Now().UnixMilli()
	key := kvKey(protocol, namespace)

	kvMu.Lock()
	defer kvMu.Unlock()

	ns := kvStore[key]
	if ns == nil {
		ns = &kvNamespace{root: &kvNode{lastAccess: now}}
		kvStore[key] = ns
	} else if now-ns.lastAccess > kvTTLMillis {
		// Namespace idle beyond TTL: all entries expired. Drop the
		// whole trie at once instead of relying on the lazy sweep.
		kvTotalNodes -= ns.nodeCount
		kvTotalTokens -= ns.tokenSum
		kvTotalBytes -= ns.byteSum
		ns = &kvNamespace{root: &kvNode{lastAccess: now}}
		kvStore[key] = ns
	}

	// Walk the trie following block hashes.
	node := ns.root
	hitBlocks := 0
	hitTokens := 0
	var hitPath []*kvNode
	hitPath = append(hitPath, node)
	missFrom := len(blocks)
	for i, b := range blocks {
		next := node.children[b.Hash]
		if next == nil {
			missFrom = i
			break
		}
		// Same hash must mean same content. On a (practically
		// impossible) FNV collision with different token counts the
		// prefix ends here instead of corrupting accounting.
		if next.tokens != b.Tokens {
			missFrom = i
			break
		}
		hitBlocks++
		hitTokens += next.tokens
		node = next
		hitPath = append(hitPath, node)
	}

	// Sliding TTL: refresh the whole hit path (root included).
	for _, n := range hitPath {
		n.lastAccess = now
	}
	ns.lastAccess = now

	// Commit the miss suffix as a new branch.
	for _, b := range blocks[missFrom:] {
		next := &kvNode{hash: b.Hash, tokens: b.Tokens, lastAccess: now}
		if node.children == nil {
			node.children = make(map[uint64]*kvNode)
		}
		node.children[b.Hash] = next
		node = next
		ns.nodeCount++
		ns.tokenSum += int64(b.Tokens)
		ns.byteSum += kvNodeBytes + int64(8)
		kvTotalNodes++
		kvTotalTokens += int64(b.Tokens)
		kvTotalBytes += kvNodeBytes + 8
	}

	// Split the billable total (content + safety margin) proportionally
	// to the content prefix ratio, so identical repeats report
	// cached == total / fresh == 0 and total == cached + fresh holds.
	cached := 0
	if contentTotal > 0 {
		cached = int(float64(out.Total)*float64(hitTokens)/float64(contentTotal) + 0.5)
		if hitTokens == contentTotal {
			cached = out.Total
		}
		if cached > out.Total {
			cached = out.Total
		}
	} else if len(blocks) == 0 {
		cached = out.Total
	}
	fresh := out.Total - cached
	if fresh < 0 {
		fresh = 0
	}

	out.Cached = cached
	out.Fresh = fresh
	out.Written = fresh
	out.Hit = cached > 0
	if out.Total > 0 {
		out.HitRatio = float64(cached) / float64(out.Total)
	}
	out.PrefixBlocks = hitBlocks

	kvCalls++
	if kvCalls%kvSweepEvery == 0 {
		kvSweepLocked(now)
		kvEvictLocked()
	}
	return out, nil
}

// KvStatsSnapshot returns a global snapshot (nodes/tokens/bytes are tracked
// incrementally; branches are counted by traversal as leaf nodes).
// It also runs the opportunistic maintenance (expiry sweep + memory-cap
// eviction), so a 1-minute polling loop doubles as the janitor: idle
// namespaces are collected even without traffic on them.
func KvStatsSnapshot() KvStats {
	kvMu.Lock()
	defer kvMu.Unlock()
	now := time.Now().UnixMilli()
	kvSweepLocked(now)
	kvEvictLocked()
	var branches int64
	for _, ns := range kvStore {
		branches += kvCountLeaves(ns.root)
	}
	avail := kvMaxBytes - kvTotalBytes
	if avail < 0 {
		avail = 0
	}
	return KvStats{
		Namespaces:     len(kvStore),
		Nodes:          kvTotalNodes,
		Branches:       branches,
		Tokens:         kvTotalTokens,
		Bytes:          kvTotalBytes,
		MaxBytes:       kvMaxBytes,
		AvailableBytes: avail,
		TTLSeconds:     kvTTLMillis / 1000,
	}
}

// KvClear removes one namespace, or everything when namespace == "".
// With protocol separation on (default), pass the plain namespace and
// all its protocol scopes are cleared; use KvClearProtocol for a
// single protocol scope.
func KvClear(namespace string) {
	kvMu.Lock()
	defer kvMu.Unlock()
	if namespace == "" {
		kvStore = make(map[string]*kvNamespace)
		kvTotalNodes, kvTotalTokens, kvTotalBytes = 0, 0, 0
		return
	}
	for _, key := range []string{namespace, "anthropic\x00" + namespace, "chat\x00" + namespace, "responses\x00" + namespace} {
		if ns := kvStore[key]; ns != nil {
			kvTotalNodes -= ns.nodeCount
			kvTotalTokens -= ns.tokenSum
			kvTotalBytes -= ns.byteSum
			delete(kvStore, key)
		}
	}
}

// KvClearProtocol removes a single protocol scope of a namespace.
func KvClearProtocol(protocol, namespace string) {
	kvMu.Lock()
	defer kvMu.Unlock()
	key := kvKey(protocol, namespace)
	if ns := kvStore[key]; ns != nil {
		kvTotalNodes -= ns.nodeCount
		kvTotalTokens -= ns.tokenSum
		kvTotalBytes -= ns.byteSum
		delete(kvStore, key)
	}
}

// kvCountLeaves counts leaf nodes (branch tips) of one trie.
func kvCountLeaves(n *kvNode) int64 {
	if n == nil {
		return 0
	}
	if len(n.children) == 0 {
		return 1
	}
	var c int64
	for _, ch := range n.children {
		c += kvCountLeaves(ch)
	}
	return c
}

// kvSubtreeCounts sums nodes/tokens/bytes of a subtree including root.
func kvSubtreeCounts(n *kvNode) (nodes, tokens, bytes int64) {
	if n == nil {
		return 0, 0, 0
	}
	nodes = 1
	tokens = int64(n.tokens)
	bytes = kvNodeBytes + 8
	for _, ch := range n.children {
		nn, tt, bb := kvSubtreeCounts(ch)
		nodes += nn
		tokens += tt
		bytes += bb
	}
	return nodes, tokens, bytes
}

// kvSweepLocked prunes expired subtrees. A parent is always at least
// as fresh as its children (hits refresh the whole path), so an
// expired parent implies an expired subtree — prune it whole.
func kvSweepLocked(now int64) {
	for name, ns := range kvStore {
		kept := make(map[uint64]*kvNode, len(ns.root.children))
		for h, ch := range ns.root.children {
			if now-ch.lastAccess > kvTTLMillis {
				nn, tt, bb := kvSubtreeCounts(ch)
				ns.nodeCount -= nn
				ns.tokenSum -= tt
				ns.byteSum -= bb
				kvTotalNodes -= nn
				kvTotalTokens -= tt
				kvTotalBytes -= bb
				continue
			}
			kvSweepNode(ch, now, ns)
			kept[h] = ch
		}
		ns.root.children = kept
		if ns.nodeCount <= 0 && len(kept) == 0 {
			delete(kvStore, name)
		}
	}
}

func kvSweepNode(n *kvNode, now int64, ns *kvNamespace) {
	if len(n.children) == 0 {
		return
	}
	kept := make(map[uint64]*kvNode, len(n.children))
	for h, ch := range n.children {
		if now-ch.lastAccess > kvTTLMillis {
			nn, tt, bb := kvSubtreeCounts(ch)
			ns.nodeCount -= nn
			ns.tokenSum -= tt
			ns.byteSum -= bb
			kvTotalNodes -= nn
			kvTotalTokens -= tt
			kvTotalBytes -= bb
			continue
		}
		kvSweepNode(ch, now, ns)
		kept[h] = ch
	}
	n.children = kept
}

// kvEvictLocked enforces the global memory cap with LRU eviction towards
// 95% of the limit: whole oldest namespaces first, then oldest leaf
// branches inside the remaining namespace(s) if a single entry alone
// still overflows. Token counts are kept for stats only.
func kvTargetBytes() int64 {
	return int64(float64(kvMaxBytes) * kvTargetRatio)
}

func kvEvictLocked() {
	targetBytes := kvTargetBytes()
	for kvTotalBytes > kvMaxBytes && kvTotalBytes > targetBytes {
		if len(kvStore) == 0 {
			break
		}
		if len(kvStore) > 1 {
			oldest := ""
			var oldestTs int64
			first := true
			for name, ns := range kvStore {
				if first || ns.lastAccess < oldestTs {
					oldest, oldestTs, first = name, ns.lastAccess, false
				}
			}
			if ns := kvStore[oldest]; ns != nil {
				kvTotalNodes -= ns.nodeCount
				kvTotalTokens -= ns.tokenSum
				kvTotalBytes -= ns.byteSum
				delete(kvStore, oldest)
				continue
			}
			break
		}
		// Single namespace overflow: prune oldest leaf branches.
		for _, ns := range kvStore {
			if !kvPruneOldestLeaves(ns, 64) {
				break
			}
		}
		break
	}
}

// kvPruneOldestLeaves removes up to maxVictims oldest leaf nodes (one
// scan per call). Returns false when there is nothing left to prune.
func kvPruneOldestLeaves(ns *kvNamespace, maxVictims int) bool {
	type victim struct {
		parent *kvNode
		hash   uint64
		ts     int64
	}
	for v := 0; v < maxVictims; v++ {
		var best *victim
		var walk func(n *kvNode, parent *kvNode, h uint64)
		walk = func(n *kvNode, parent *kvNode, h uint64) {
			if len(n.children) == 0 {
				if parent != nil && (best == nil || n.lastAccess < best.ts) {
					best = &victim{parent: parent, hash: h, ts: n.lastAccess}
				}
				return
			}
			for h, ch := range n.children {
				walk(ch, n, h)
			}
		}
		walk(ns.root, nil, 0)
		if best == nil {
			return false
		}
		leaf := best.parent.children[best.hash]
		if leaf == nil {
			return false
		}
		delete(best.parent.children, best.hash)
		ns.nodeCount--
		ns.tokenSum -= int64(leaf.tokens)
		ns.byteSum -= kvNodeBytes
		kvTotalNodes--
		kvTotalTokens -= int64(leaf.tokens)
		kvTotalBytes -= kvNodeBytes
		if ns.nodeCount <= 0 {
			for name, cand := range kvStore {
				if cand == ns {
					delete(kvStore, name)
					break
				}
			}
			return false
		}
		if kvTotalBytes <= kvTargetBytes() {
			return false
		}
	}
	return true
}

// kvLinearize parses the payload by protocol and flattens it into the
// canonical block sequence, alongside the standard Breakdown (which
// carries the safety margin). Block token sums match
// System+Messages+Tools of the Breakdown.
func kvLinearize(raw []byte, protocol string, opts Options) ([]KvBlock, Breakdown, error) {
	switch protocol {
	case "anthropic":
		var req AnthropicRequest
		if err := json.UnmarshalNoEscape(raw, &req); err != nil {
			return nil, Breakdown{}, err
		}
		// Fused single walk: count + linearize together.
		bd, blocks, err := kvCountAnthropicRequest(req, opts)
		if err != nil {
			return nil, bd, err
		}
		return blocks, bd, nil
	case "chat":
		var req ChatRequest
		if err := json.UnmarshalNoEscape(raw, &req); err != nil {
			return nil, Breakdown{}, err
		}
		// Fused single walk: count + linearize together (not two passes).
		bd, blocks, err := kvCountChatRequest(req, opts)
		if err != nil {
			return nil, bd, err
		}
		return blocks, bd, nil
	case "responses":
		var req ResponsesRequest
		if err := json.UnmarshalNoEscape(raw, &req); err != nil {
			return nil, Breakdown{}, err
		}
		// Fused single walk: count + linearize together.
		bd, blocks, err := kvCountResponsesRequest(req, opts)
		if err != nil {
			return nil, bd, err
		}
		return blocks, bd, nil
	default:
		// Small alias tolerance for gateway callers.
		switch {
		case protocol == "anthropic-messages" || protocol == "claude":
			return kvLinearize(raw, "anthropic", opts)
		case protocol == "chat-completions" || protocol == "openai-chat" || protocol == "openai":
			return kvLinearize(raw, "chat", opts)
		case protocol == "openai-responses" || protocol == "response":
			return kvLinearize(raw, "responses", opts)
		}
		return nil, Breakdown{}, errUnknownProtocol(protocol)
	}
}

// --- FNV-1a helpers (fast, no alloc beyond the hashed strings) ---

func kvHashNew() uint64 { return 14695981039346656037 }

func kvHashStr(h uint64, s string) uint64 {
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= 1099511628211
	}
	return h
}

func kvHashSep(h uint64) uint64 {
	h ^= 0xFF
	h *= 1099511628211
	return h
}

func kvHashInt(h uint64, n int) uint64 {
	var buf [8]byte
	v := uint64(n)
	for i := 0; i < 8; i++ {
		buf[i] = byte(v >> (8 * i))
	}
	for _, b := range buf {
		h ^= uint64(b)
		h *= 1099511628211
	}
	return h
}

// --- Anthropic linearization ---

func kvAnthropicBlocks(req AnthropicRequest, opts Options) []KvBlock {
	bd, blocks, err := kvCountAnthropicRequest(req, opts)
	if err != nil {
		return nil
	}
	_ = bd
	return blocks
}

// kvCountAnthropicRequest counts + linearizes a decoded Anthropic request
// in ONE walk. Shared by CountAnthropicRequest (drops blocks) and
// KvLookup (keeps both). Breakdown identical to CountAnthropicRequest
// (sequential path; the >=32 parallel path is only for struct callers).
func kvCountAnthropicRequest(req AnthropicRequest, opts Options) (Breakdown, []KvBlock, error) {
	var out Breakdown
	var blocks []KvBlock
	if req.System != nil {
		blocks = kvAppendTextValue(blocks, "sys", "", req.System, opts)
		sum := 0
		for _, b := range blocks {
			sum += b.Tokens
		}
		out.System = sum
	}
	out.ByMessage = make([]int, len(req.Messages))
	for i, m := range req.Messages {
		before := len(blocks)
		var n, imgs, imgCount int
		blocks, n, imgs, imgCount = kvAppendAnthropicMessageCount(blocks, m, opts)
		out.ByMessage[i] = n
		out.Messages += n
		out.Images += imgs
		out.ImageCount += imgCount
		_ = before
	}
	out.ByTool = make([]int, 0, len(req.Tools))
	for _, t := range req.Tools {
		schemaStr := jsonString(t.InputSchema)
		n, err := countToolFast(t, schemaStr)
		if err != nil {
			return out, nil, err
		}
		h := kvHashNew()
		h = kvHashStr(h, "tool")
		h = kvHashSep(h)
		h = kvHashStr(h, t.Name)
		h = kvHashSep(h)
		h = kvHashStr(h, t.Description)
		h = kvHashSep(h)
		h = kvHashStr(h, schemaStr)
		h = kvHashInt(h, n)
		blocks = append(blocks, KvBlock{Hash: h, Tokens: n})
		out.ByTool = append(out.ByTool, n)
		out.Tools += n
	}
	out.TextTokens = out.System + out.Messages + out.Tools - out.Images
	out.Total = out.System + out.Messages + out.Tools
	applySafetyMargin(&out, opts, len(req.Messages), len(req.Tools), out.ImageCount)
	return out, blocks, nil
}

// countToolFast counts a tool given its pre-marshaled schema string
// (avoids marshaling twice when the KV path needs it for hashing).
func countToolFast(t AnthropicTool, schemaStr string) (int, error) {
	total := 0
	n, err := codec.FastCountNoErr(t.Name)
	if err != nil {
		return 0, err
	}
	total += n
	if t.Description != "" {
		n, err := codec.FastCountNoErr(t.Description)
		if err != nil {
			return 0, err
		}
		total += n
	}
	if t.InputSchema != nil {
		n, err := codec.FastCountNoErr(schemaStr)
		if err != nil {
			return 0, err
		}
		total += n
	}
	return total, nil
}

// kvAppendAnthropicMessageCount is the fused version: appends blocks AND
// returns (blocks, tokens, images, imageBlocks) so the single walk
// produces a Breakdown identical to countMessage + imageBlocksIn.
func kvAppendAnthropicMessageCount(out []KvBlock, m AnthropicMessage, opts Options) ([]KvBlock, int, int, int) {
	role := m.Role
	switch c := m.Content.(type) {
	case nil:
		return out, 0, 0, 0
	case string:
		n, _ := codec.FastCountNoErr(c)
		h := kvHashNew()
		h = kvHashStr(h, "msg")
		h = kvHashSep(h)
		h = kvHashStr(h, role)
		h = kvHashSep(h)
		h = kvHashStr(h, c)
		h = kvHashInt(h, n)
		return append(out, KvBlock{Hash: h, Tokens: n}), n, 0, 0
	case []any:
		total, imgToks, imgCount := 0, 0, 0
		for _, b := range c {
			bm, ok := b.(map[string]any)
			if !ok {
				s := jsonString(b)
				n, _ := codec.FastCountNoErr(s)
				h := kvHashNew()
				h = kvHashStr(h, "blk")
				h = kvHashSep(h)
				h = kvHashStr(h, role)
				h = kvHashSep(h)
				h = kvHashStr(h, s)
				h = kvHashInt(h, n)
				out = append(out, KvBlock{Hash: h, Tokens: n})
				total += n
				continue
			}
			var n, it, cnt int
			out, n, it, cnt = kvAppendAnthropicBlockCount(out, role, bm, opts)
			total += n
			imgToks += it
			imgCount += cnt
		}
		return out, total, imgToks, imgCount
	default:
		s := jsonString(c)
		n, _ := codec.FastCountNoErr(s)
		h := kvHashNew()
		h = kvHashStr(h, "msg")
		h = kvHashSep(h)
		h = kvHashStr(h, role)
		h = kvHashSep(h)
		h = kvHashStr(h, s)
		h = kvHashInt(h, n)
		return append(out, KvBlock{Hash: h, Tokens: n}), n, 0, 0
	}
}


// kvAppendAnthropicBlockCount is the fused version of countContentBlock +
// kvAppendAnthropicBlock: appends the block AND returns
// (blocks, tokens, images, imageBlocks).
func kvAppendAnthropicBlockCount(out []KvBlock, role string, bm map[string]any, opts Options) ([]KvBlock, int, int, int) {
	typ, _ := bm["type"].(string)
	mk := func(kind string, parts ...string) uint64 {
		h := kvHashNew()
		h = kvHashStr(h, kind)
		for _, p := range parts {
			h = kvHashSep(h)
			h = kvHashStr(h, p)
		}
		return h
	}
	switch typ {
	case "text":
		s := strField(bm, "text")
		n, _ := codec.FastCountNoErr(s)
		h := kvHashInt(mk("text", role, s), n)
		return append(out, KvBlock{Hash: h, Tokens: n}), n, 0, 0
	case "image":
		var it int
		var canon string
		if opts.IgnoreImages {
			it = 0
			canon = "ignored"
		} else {
			it = countImageSource(bm["source"])
			canon = kvImageCanon(bm["source"], it)
		}
		h := kvHashInt(mk("image", role, canon), it)
		return append(out, KvBlock{Hash: h, Tokens: it}), it, it, 1
	case "tool_use":
		name := strField(bm, "name")
		var inputStr string
		hasInput := false
		if in, ok := bm["input"]; ok {
			inputStr = jsonString(in)
			hasInput = true
		}
		nName, _ := codec.FastCountNoErr(name)
		n := nName
		if hasInput {
			nIn, _ := codec.FastCountNoErr(inputStr)
			n += nIn
		}
		h := kvHashNew()
		h = kvHashStr(h, "tool_use")
		h = kvHashSep(h)
		h = kvHashStr(h, role)
		h = kvHashSep(h)
		h = kvHashStr(h, name)
		h = kvHashSep(h)
		h = kvHashStr(h, inputStr)
		h = kvHashInt(h, n)
		return append(out, KvBlock{Hash: h, Tokens: n}), n, 0, 0
	case "tool_result":
		return kvAppendToolResultCount(out, role, bm["content"], opts)
	case "thinking":
		s := strField(bm, "thinking")
		n, _ := codec.FastCountNoErr(s)
		h := kvHashInt(mk("thinking", role, s), n)
		return append(out, KvBlock{Hash: h, Tokens: n}), n, 0, 0
	case "redacted_thinking":
		d, _ := bm["data"].(string)
		n := 0
		if d != "" {
			n = EstimateThinkingTokens(d)
		}
		h := kvHashInt(mk("redacted", role, d), n)
		return append(out, KvBlock{Hash: h, Tokens: n}), n, 0, 0
	default:
		s := jsonString(bm)
		n, _ := codec.FastCountNoErr(s)
		h := kvHashNew()
		h = kvHashStr(h, "blk")
		h = kvHashSep(h)
		h = kvHashStr(h, role)
		h = kvHashSep(h)
		h = kvHashStr(h, typ)
		h = kvHashSep(h)
		h = kvHashStr(h, s)
		h = kvHashInt(h, n)
		return append(out, KvBlock{Hash: h, Tokens: n}), n, 0, 0
	}
}

// kvAppendToolResultCount is the fused version of countToolResult +
// kvAppendToolResult: appends blocks AND returns
// (blocks, tokens, images, imageBlocks).
func kvAppendToolResultCount(out []KvBlock, role string, c any, opts Options) ([]KvBlock, int, int, int) {
	switch t := c.(type) {
	case nil:
		return out, 0, 0, 0
	case string:
		n, _ := codec.FastCountNoErr(t)
		h := kvHashNew()
		h = kvHashStr(h, "tool_result")
		h = kvHashSep(h)
		h = kvHashStr(h, role)
		h = kvHashSep(h)
		h = kvHashStr(h, t)
		h = kvHashInt(h, n)
		return append(out, KvBlock{Hash: h, Tokens: n}), n, 0, 0
	case []any:
		total, imgToks, imgCount := 0, 0, 0
		for _, b := range t {
			bm, ok := b.(map[string]any)
			if !ok {
				s := jsonString(b)
				n, _ := codec.FastCountNoErr(s)
				h := kvHashNew()
				h = kvHashStr(h, "tool_result")
				h = kvHashSep(h)
				h = kvHashStr(h, role)
				h = kvHashSep(h)
				h = kvHashStr(h, s)
				h = kvHashInt(h, n)
				out = append(out, KvBlock{Hash: h, Tokens: n})
				total += n
				continue
			}
			// Nested blocks keep their own identity but stay scoped
			// under the tool_result position.
			var n, it, cnt int
			out, n, it, cnt = kvAppendAnthropicBlockCount(out, role+"|tool_result", bm, opts)
			total += n
			imgToks += it
			imgCount += cnt
		}
		return out, total, imgToks, imgCount
	default:
		s := jsonString(c)
		n, _ := codec.FastCountNoErr(s)
		h := kvHashNew()
		h = kvHashStr(h, "tool_result")
		h = kvHashSep(h)
		h = kvHashStr(h, role)
		h = kvHashSep(h)
		h = kvHashStr(h, s)
		h = kvHashInt(h, n)
		return append(out, KvBlock{Hash: h, Tokens: n}), n, 0, 0
	}
}


// kvAppendTextValue flattens system/instruction values with the same
// token math as countTextValue.
func kvAppendTextValue(out []KvBlock, kind, role string, v any, opts Options) []KvBlock {
	switch t := v.(type) {
	case nil:
		return out
	case string:
		n, _ := codec.FastCountNoErr(t)
		h := kvHashNew()
		h = kvHashStr(h, kind)
		h = kvHashSep(h)
		h = kvHashStr(h, role)
		h = kvHashSep(h)
		h = kvHashStr(h, t)
		h = kvHashInt(h, n)
		return append(out, KvBlock{Hash: h, Tokens: n})
	case []any:
		for _, b := range t {
			if bm, ok := b.(map[string]any); ok && bm["type"] == "text" {
				s := strField(bm, "text")
				n, _ := codec.FastCountNoErr(s)
				h := kvHashNew()
				h = kvHashStr(h, kind)
				h = kvHashSep(h)
				h = kvHashStr(h, role)
				h = kvHashSep(h)
				h = kvHashStr(h, s)
				h = kvHashInt(h, n)
				out = append(out, KvBlock{Hash: h, Tokens: n})
			} else {
				s := jsonString(b)
				n, _ := codec.FastCountNoErr(s)
				h := kvHashNew()
				h = kvHashStr(h, kind)
				h = kvHashSep(h)
				h = kvHashStr(h, role)
				h = kvHashSep(h)
				h = kvHashStr(h, s)
				h = kvHashInt(h, n)
				out = append(out, KvBlock{Hash: h, Tokens: n})
			}
		}
		return out
	default:
		if m, ok := v.(map[string]any); ok {
			if s, ok := m["text"].(string); ok {
				n, _ := codec.FastCountNoErr(s)
				h := kvHashNew()
				h = kvHashStr(h, kind)
				h = kvHashSep(h)
				h = kvHashStr(h, role)
				h = kvHashSep(h)
				h = kvHashStr(h, s)
				h = kvHashInt(h, n)
				return append(out, KvBlock{Hash: h, Tokens: n})
			}
		}
		s := jsonString(v)
		n, _ := codec.FastCountNoErr(s)
		h := kvHashNew()
		h = kvHashStr(h, kind)
		h = kvHashSep(h)
		h = kvHashStr(h, role)
		h = kvHashSep(h)
		h = kvHashStr(h, s)
		h = kvHashInt(h, n)
		return append(out, KvBlock{Hash: h, Tokens: n})
	}
}

func errUnknownProtocol(protocol string) error {
	return errors.New("btdby4: unknown kv protocol " + protocol + " (want anthropic|chat|responses)")
}

// kvB64Dims decodes just enough of a base64 payload to read the image
// header for identity (no token math here).
func kvB64Dims(s string) (int, int, bool) {
	if i := strings.Index(s, ","); i >= 0 {
		s = s[i+1:]
	}
	s = strings.TrimSpace(s)
	if len(s) > 176 {
		s = s[:176]
	}
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return 0, 0, false
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(b))
	if err != nil {
		return 0, 0, false
	}
	return cfg.Width, cfg.Height, true
}

// kvImageCanon identifies an image without hashing megabytes:
// dims when decodable, else length + head/tail sample of the payload.
func kvImageCanon(src any, tokens int) string {
	sm, ok := src.(map[string]any)
	if !ok {
		return "unknown"
	}
	if d, ok := sm["data"].(string); ok && d != "" {
		if w, h, ok := kvB64Dims(d); ok {
			return "b64dims:" + itoa(w) + "x" + itoa(h) + "#" + itoa(len(d))
		}
		head := d
		if len(head) > 512 {
			head = head[:512]
		}
		tail := ""
		if len(d) > 512 {
			tail = d[len(d)-128:]
		}
		return "b64:" + itoa(len(d)) + ":" + head + ":" + tail
	}
	if u, ok := sm["url"].(string); ok && u != "" {
		return "url:" + u
	}
	return "empty#" + itoa(tokens)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// --- Chat linearization ---
// NOTE: chat hot path is fused in kvCountChatRequest (single walk).
// kvChatBlocks stays for tests/debug only.

func kvChatBlocks(req ChatRequest, opts Options) []KvBlock {
	var out []KvBlock
	out = kvAppendTextValue(out, "sys", "", req.System, opts)
	for _, t := range req.Tools {
		h := kvHashNew()
		h = kvHashStr(h, "tool")
		h = kvHashSep(h)
		h = kvHashStr(h, t.Function.Name)
		h = kvHashSep(h)
		h = kvHashStr(h, t.Function.Description)
		h = kvHashSep(h)
		h = kvHashStr(h, jsonString(t.Function.Parameters))
		n, _ := countChatTool(t)
		h = kvHashInt(h, n)
		out = append(out, KvBlock{Hash: h, Tokens: n})
	}
	for _, m := range req.Messages {
		out = kvAppendChatMessageCount(out, m, opts, &Breakdown{})
	}
	return out
}

// kvAppendChatMessageCount is kvAppendChatMessage + image accounting in one
// pass: it also accumulates out.Images / out.ImageCount so the fused walk
// produces a Breakdown identical to countChatMessage/chatImagesIn.
func kvAppendChatMessageCount(out []KvBlock, m ChatMessage, opts Options, acc *Breakdown) []KvBlock {
	role := m.Role
	if m.ToolCallID != "" {
		switch c := m.Content.(type) {
		case nil:
			return out
		case string:
			n, _ := codec.FastCountNoErr(c)
			h := kvHashNew()
			h = kvHashStr(h, "toolmsg")
			h = kvHashSep(h)
			h = kvHashStr(h, role)
			h = kvHashSep(h)
			h = kvHashStr(h, m.ToolCallID)
			h = kvHashSep(h)
			h = kvHashStr(h, c)
			h = kvHashInt(h, n)
			return append(out, KvBlock{Hash: h, Tokens: n})
		default:
			s := jsonString(c)
			n, _ := codec.FastCountNoErr(s)
			h := kvHashNew()
			h = kvHashStr(h, "toolmsg")
			h = kvHashSep(h)
			h = kvHashStr(h, role)
			h = kvHashSep(h)
			h = kvHashStr(h, m.ToolCallID)
			h = kvHashSep(h)
			h = kvHashStr(h, s)
			h = kvHashInt(h, n)
			return append(out, KvBlock{Hash: h, Tokens: n})
		}
	}
	// tool_calls are ordered positions after content, like the wire.
	flushCalls := func(out []KvBlock) []KvBlock {
		for _, tc := range m.ToolCalls {
			nName, _ := codec.FastCountNoErr(tc.Function.Name)
			nArgs, _ := codec.FastCountNoErr(tc.Function.Arguments)
			n := nName + nArgs
			h := kvHashNew()
			h = kvHashStr(h, "tool_call")
			h = kvHashSep(h)
			h = kvHashStr(h, role)
			h = kvHashSep(h)
			h = kvHashStr(h, tc.ID)
			h = kvHashSep(h)
			h = kvHashStr(h, tc.Function.Name)
			h = kvHashSep(h)
			h = kvHashStr(h, tc.Function.Arguments)
			h = kvHashInt(h, n)
			out = append(out, KvBlock{Hash: h, Tokens: n})
		}
		return out
	}
	switch c := m.Content.(type) {
	case nil:
		return flushCalls(out)
	case string:
		n, _ := codec.FastCountNoErr(c)
		h := kvHashNew()
		h = kvHashStr(h, "msg")
		h = kvHashSep(h)
		h = kvHashStr(h, role)
		h = kvHashSep(h)
		if m.Name != "" {
			h = kvHashStr(h, m.Name)
			h = kvHashSep(h)
		}
		h = kvHashStr(h, c)
		h = kvHashInt(h, n)
		out = append(out, KvBlock{Hash: h, Tokens: n})
		return flushCalls(out)
	case []any:
		for _, b := range c {
			bm, ok := b.(map[string]any)
			if !ok {
				s := jsonString(b)
				n, _ := codec.FastCountNoErr(s)
				h := kvHashNew()
				h = kvHashStr(h, "part")
				h = kvHashSep(h)
				h = kvHashStr(h, role)
				h = kvHashSep(h)
				h = kvHashStr(h, s)
				h = kvHashInt(h, n)
				out = append(out, KvBlock{Hash: h, Tokens: n})
				continue
			}
			out = kvAppendChatPartCount(out, role, bm, opts, acc)
		}
		return flushCalls(out)
	default:
		s := jsonString(c)
		n, _ := codec.FastCountNoErr(s)
		h := kvHashNew()
		h = kvHashStr(h, "msg")
		h = kvHashSep(h)
		h = kvHashStr(h, role)
		h = kvHashSep(h)
		h = kvHashStr(h, s)
		h = kvHashInt(h, n)
		out = append(out, KvBlock{Hash: h, Tokens: n})
		return flushCalls(out)
	}
}

func kvAppendChatPartCount(out []KvBlock, role string, bm map[string]any, opts Options, acc *Breakdown) []KvBlock {
	typ, _ := bm["type"].(string)
	switch typ {
	case "text", "reasoning_content":
		s := strField(bm, "text")
		n, _ := codec.FastCountNoErr(s)
		h := kvHashNew()
		h = kvHashStr(h, "text")
		h = kvHashSep(h)
		h = kvHashStr(h, role)
		h = kvHashSep(h)
		h = kvHashStr(h, s)
		h = kvHashInt(h, n)
		return append(out, KvBlock{Hash: h, Tokens: n})
	case "image_url":
		if opts.IgnoreImages {
			h := kvHashNew()
			h = kvHashStr(h, "image")
			h = kvHashSep(h)
			h = kvHashStr(h, role)
			h = kvHashSep(h)
			h = kvHashStr(h, "ignored")
			h = kvHashInt(h, 0)
			return append(out, KvBlock{Hash: h, Tokens: 0})
		}
		var url string
		if inner, ok := bm["image_url"].(map[string]any); ok {
			url, _ = inner["url"].(string)
		} else {
			url, _ = bm["url"].(string)
		}
		it := countChatImageURL(url)
		acc.Images += it
		acc.ImageCount++
		h := kvHashNew()
		h = kvHashStr(h, "image")
		h = kvHashSep(h)
		h = kvHashStr(h, role)
		h = kvHashSep(h)
		h = kvHashStr(h, kvURLCanon(url))
		h = kvHashInt(h, it)
		return append(out, KvBlock{Hash: h, Tokens: it})
	case "input_audio":
		var data string
		if inner, ok := bm["input_audio"].(map[string]any); ok {
			data, _ = inner["data"].(string)
		}
		var n int
		var canon string
		if data != "" {
			n, _ = codec.FastCountNoErr(data)
			canon = data
		} else {
			s := jsonString(bm)
			n, _ = codec.FastCountNoErr(s)
			canon = s
		}
		h := kvHashNew()
		h = kvHashStr(h, "audio")
		h = kvHashSep(h)
		h = kvHashStr(h, role)
		h = kvHashSep(h)
		h = kvHashStr(h, canon)
		h = kvHashInt(h, n)
		return append(out, KvBlock{Hash: h, Tokens: n})
	default:
		s := jsonString(bm)
		n, _ := codec.FastCountNoErr(s)
		h := kvHashNew()
		h = kvHashStr(h, "part")
		h = kvHashSep(h)
		h = kvHashStr(h, role)
		h = kvHashSep(h)
		h = kvHashStr(h, typ)
		h = kvHashSep(h)
		h = kvHashStr(h, s)
		h = kvHashInt(h, n)
		return append(out, KvBlock{Hash: h, Tokens: n})
	}
}

func kvURLCanon(url string) string {
	if url == "" {
		return "empty"
	}
	if idx := indexOf(url, ";base64,"); idx >= 0 {
		payload := url[idx+8:]
		if w, h, ok := kvB64Dims(payload); ok {
			return "b64dims:" + itoa(w) + "x" + itoa(h) + "#" + itoa(len(payload))
		}
		head := payload
		if len(head) > 512 {
			head = head[:512]
		}
		tail := ""
		if len(payload) > 512 {
			tail = payload[len(payload)-128:]
		}
		return "b64:" + itoa(len(payload)) + ":" + head + ":" + tail
	}
	return "url:" + url
}

// kvResponsesBlocks flattens in real KV order: instructions, tools,
// then input. NOTE: the request shape lists tools last, but they sit
// before input in the KV prefix — order matters for hit fidelity.
// The fused counter below produces the same sequence; this stays for
// tests/debug only.
func kvResponsesBlocks(req ResponsesRequest, opts Options) []KvBlock {
	_, blocks, err := kvCountResponsesRequest(req, opts)
	if err != nil {
		return nil
	}
	return blocks
}

// kvCountResponsesRequest counts + linearizes a decoded Responses request
// in ONE walk. Shared by CountResponsesRequest (drops blocks) and
// KvLookup (keeps both). Breakdown identical to CountResponsesRequest.
func kvCountResponsesRequest(req ResponsesRequest, opts Options) (Breakdown, []KvBlock, error) {
	var out Breakdown
	var think thinkingAccumulator
	var sys []KvBlock
	if req.Instructions != nil {
		sys = kvAppendResponsesTextCount(sys, "sys", req.Instructions, opts)
		sum := 0
		for _, b := range sys {
			sum += b.Tokens
		}
		out.System = sum
	}
	var tools []KvBlock
	var toolsTotal int
	out.ByTool = make([]int, 0, len(req.Tools))
	for _, t := range req.Tools {
		paramStr := jsonString(t.Parameters)
		n, err := countResponsesToolFast(t, paramStr)
		if err != nil {
			return out, nil, err
		}
		h := kvHashNew()
		h = kvHashStr(h, "tool")
		h = kvHashSep(h)
		h = kvHashStr(h, t.Type)
		h = kvHashSep(h)
		h = kvHashStr(h, t.Name)
		h = kvHashSep(h)
		h = kvHashStr(h, t.Description)
		h = kvHashSep(h)
		h = kvHashStr(h, paramStr)
		h = kvHashInt(h, n)
		tools = append(tools, KvBlock{Hash: h, Tokens: n})
		out.ByTool = append(out.ByTool, n)
		toolsTotal += n
	}
	var input []KvBlock
	items, ok := req.Input.([]any)
	if !ok {
		if req.Input != nil {
			input = kvAppendResponsesTextCount(input, "in", req.Input, opts)
			sum := 0
			for _, b := range input {
				sum += b.Tokens
			}
			out.Messages = sum
			out.ByMessage = []int{sum}
		}
		out.Tools = toolsTotal
		applyThinking(&out, req, opts, &think)
		return finishResponsesFused(&out, req, opts, sys, tools, input)
	}
	out.ByMessage = make([]int, len(items))
	for i, raw := range items {
		it, ok := raw.(map[string]any)
		if !ok {
			s := jsonString(raw)
			n, _ := codec.FastCountNoErr(s)
			h := kvHashNew()
			h = kvHashStr(h, "item")
			h = kvHashSep(h)
			h = kvHashStr(h, s)
			h = kvHashInt(h, n)
			input = append(input, KvBlock{Hash: h, Tokens: n})
			out.ByMessage[i] = n
			out.Messages += n
			continue
		}
		var n, imgs, cnt int
		input, n, imgs, cnt = kvAppendResponsesItemCount(input, it, opts, &think)
		out.ByMessage[i] = n
		out.Messages += n
		out.Images += imgs
		out.ImageCount += cnt
	}
	out.Tools = toolsTotal
	applyThinking(&out, req, opts, &think)
	return finishResponsesFused(&out, req, opts, sys, tools, input)
}

// finishResponsesFused assembles sys+tools+input order (real KV order)
// and applies the shared tool/image accounting of finishResponses.
// NOTE: the margin uses len(ByMessage) like the count path, so totals
// match CountResponsesRequest exactly.
func finishResponsesFused(out *Breakdown, req ResponsesRequest, opts Options, sys, tools, input []KvBlock) (Breakdown, []KvBlock, error) {
	out.TextTokens = out.System + out.Messages + out.Tools - out.Images
	out.Total = out.System + out.Messages + out.Tools
	applySafetyMargin(out, opts, len(out.ByMessage), len(req.Tools), out.ImageCount)
	blocks := make([]KvBlock, 0, len(sys)+len(tools)+len(input))
	blocks = append(blocks, sys...)
	blocks = append(blocks, tools...)
	blocks = append(blocks, input...)
	return *out, blocks, nil
}

// countResponsesToolFast counts a tool given its pre-marshaled params
// string (avoids marshaling twice when the KV path needs it for hashing).
func countResponsesToolFast(t ResponsesTool, paramStr string) (int, error) {
	if t.Type != "function" {
		return codec.FastCountNoErr(paramStrFull(t, paramStr))
	}
	total := 0
	n, err := codec.FastCountNoErr(t.Name)
	if err != nil {
		return 0, err
	}
	total += n
	if t.Description != "" {
		n, err := codec.FastCountNoErr(t.Description)
		if err != nil {
			return 0, err
		}
		total += n
	}
	if t.Parameters != nil {
		n, err := codec.FastCountNoErr(paramStr)
		if err != nil {
			return 0, err
		}
		total += n
	}
	return total, nil
}

// paramStrFull returns the full-tool JSON for non-function tools
// (paramStr only holds Parameters, so re-marshal the whole tool).
func paramStrFull(t ResponsesTool, _ string) string {
	return jsonString(t)
}

// kvResponsesBlocksLegacy is superseded by kvCountResponsesRequest.

// kvAppendResponsesTextCount is kvAppendResponsesText (same token math as
// countResponsesText, block-by-block). Counting and hashing already happen
// together here, so the fused path reuses it directly.
func kvAppendResponsesTextCount(out []KvBlock, kind string, v any, opts Options) []KvBlock {
	return kvAppendResponsesText(out, kind, v, opts)
}

func kvAppendResponsesText(out []KvBlock, kind string, v any, opts Options) []KvBlock {
	// Same math as countResponsesText, but block-by-block.
	switch t := v.(type) {
	case nil:
		return out
	case string:
		n, _ := codec.FastCountNoErr(t)
		h := kvHashNew()
		h = kvHashStr(h, kind)
		h = kvHashSep(h)
		h = kvHashStr(h, t)
		h = kvHashInt(h, n)
		return append(out, KvBlock{Hash: h, Tokens: n})
	case []any:
		for _, p := range t {
			out = kvAppendResponsesText(out, kind, p, opts)
		}
		return out
	default:
		if m, ok := v.(map[string]any); ok {
			if s, ok := m["text"].(string); ok {
				n, _ := codec.FastCountNoErr(s)
				h := kvHashNew()
				h = kvHashStr(h, kind)
				h = kvHashSep(h)
				h = kvHashStr(h, s)
				h = kvHashInt(h, n)
				return append(out, KvBlock{Hash: h, Tokens: n})
			}
		}
		s := jsonString(v)
		n, _ := codec.FastCountNoErr(s)
		h := kvHashNew()
		h = kvHashStr(h, kind)
		h = kvHashSep(h)
		h = kvHashStr(h, s)
		h = kvHashInt(h, n)
		return append(out, KvBlock{Hash: h, Tokens: n})
	}
}

// kvAppendResponsesItemCount is the fused version of countResponsesItemAcc +
// kvAppendResponsesItem: appends blocks AND returns
// (blocks, tokens, images, imgCount). The thinking accumulator is
// threaded through for encrypted reasoning, like the count path.
func kvAppendResponsesItemCount(out []KvBlock, it map[string]any, opts Options, acc *thinkingAccumulator) ([]KvBlock, int, int, int) {
	typ, _ := it["type"].(string)
	switch typ {
	case "message":
		total, imgToks, imgCount := 0, 0, 0
		for _, raw := range asAnyArr(it["content"]) {
			p, ok := raw.(map[string]any)
			if !ok {
				s := jsonString(raw)
				n, _ := codec.FastCountNoErr(s)
				h := kvHashNew()
				h = kvHashStr(h, "msgpart")
				h = kvHashSep(h)
				h = kvHashStr(h, s)
				h = kvHashInt(h, n)
				out = append(out, KvBlock{Hash: h, Tokens: n})
				total += n
				continue
			}
			var n, it2, c int
			out, n, it2, c = kvAppendResponsesPartCount(out, p, opts)
			total += n
			imgToks += it2
			imgCount += c
		}
		return out, total, imgToks, imgCount
	case "function_call":
		name := strField(it, "name")
		args := strField(it, "arguments")
		nName, _ := codec.FastCountNoErr(name)
		nArgs, _ := codec.FastCountNoErr(args)
		n := nName + nArgs
		h := kvHashNew()
		h = kvHashStr(h, "function_call")
		h = kvHashSep(h)
		h = kvHashStr(h, name)
		h = kvHashSep(h)
		h = kvHashStr(h, args)
		h = kvHashInt(h, n)
		return append(out, KvBlock{Hash: h, Tokens: n}), n, 0, 0
	case "function_call_output":
		s := callOutputString(it["output"])
		n, _ := codec.FastCountNoErr(s)
		h := kvHashNew()
		h = kvHashStr(h, "function_call_output")
		h = kvHashSep(h)
		h = kvHashStr(h, s)
		h = kvHashInt(h, n)
		return append(out, KvBlock{Hash: h, Tokens: n}), n, 0, 0
	case "reasoning":
		total, imgToks, imgCount := 0, 0, 0
		for _, raw := range asAnyArr(it["summary"]) {
			if p, ok := raw.(map[string]any); ok {
				s := strField(p, "text")
				n, _ := codec.FastCountNoErr(s)
				h := kvHashNew()
				h = kvHashStr(h, "summary")
				h = kvHashSep(h)
				h = kvHashStr(h, s)
				h = kvHashInt(h, n)
				out = append(out, KvBlock{Hash: h, Tokens: n})
				total += n
			}
		}
		if enc, ok := it["encrypted_content"].(string); ok && enc != "" {
			if acc != nil {
				acc.add(enc)
			}
			th := singleEnvelopeEstimate(enc)
			h := kvHashNew()
			h = kvHashStr(h, "encrypted")
			h = kvHashSep(h)
			h = kvHashStr(h, enc)
				h = kvHashInt(h, th)
			out = append(out, KvBlock{Hash: h, Tokens: th})
			total += th
		}
		return out, total, imgToks, imgCount
	default:
		s := jsonString(it)
		n, _ := codec.FastCountNoErr(s)
		h := kvHashNew()
		h = kvHashStr(h, "item")
		h = kvHashSep(h)
		h = kvHashStr(h, typ)
		h = kvHashSep(h)
		h = kvHashStr(h, s)
		h = kvHashInt(h, n)
		return append(out, KvBlock{Hash: h, Tokens: n}), n, 0, 0
	}
}

// kvAppendResponsesPartCount is the fused version of countResponsesPart +
// kvAppendResponsesPart: appends the block AND returns
// (blocks, tokens, images, imgCount).
func kvAppendResponsesPartCount(out []KvBlock, part map[string]any, opts Options) ([]KvBlock, int, int, int) {
	pt, _ := part["type"].(string)
	switch pt {
	case "input_text", "output_text", "text", "reasoning_text", "reasoning_content", "summary_text":
		s := strField(part, "text")
		n, _ := codec.FastCountNoErr(s)
		h := kvHashNew()
		h = kvHashStr(h, "text")
		h = kvHashSep(h)
		h = kvHashStr(h, pt)
		h = kvHashSep(h)
		h = kvHashStr(h, s)
		h = kvHashInt(h, n)
		return append(out, KvBlock{Hash: h, Tokens: n}), n, 0, 0
	case "input_image":
		if opts.IgnoreImages {
			h := kvHashNew()
			h = kvHashStr(h, "image")
			h = kvHashSep(h)
			h = kvHashStr(h, "ignored")
			h = kvHashInt(h, 0)
			return append(out, KvBlock{Hash: h, Tokens: 0}), 0, 0, 0
		}
		var url string
		url, _ = part["image_url"].(string)
		if url == "" {
			url, _ = part["file_id"].(string)
		}
		it := countChatImageURL(url)
		h := kvHashNew()
		h = kvHashStr(h, "image")
		h = kvHashSep(h)
		h = kvHashStr(h, kvURLCanon(url))
		h = kvHashInt(h, it)
		return append(out, KvBlock{Hash: h, Tokens: it}), it, it, 1
	default:
		s := jsonString(part)
		n, _ := codec.FastCountNoErr(s)
		h := kvHashNew()
		h = kvHashStr(h, "part")
		h = kvHashSep(h)
		h = kvHashStr(h, pt)
		h = kvHashSep(h)
		h = kvHashStr(h, s)
		h = kvHashInt(h, n)
		return append(out, KvBlock{Hash: h, Tokens: n}), n, 0, 0
	}
}
