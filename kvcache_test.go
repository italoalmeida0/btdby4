package btdby4

import (
	json "github.com/goccy/go-json"
	"testing"
)

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestKvIdenticalRepeatFullHit(t *testing.T) {
	KvClear("")
	payload := map[string]any{
		"system": "You are helpful.",
		"messages": []any{
			map[string]any{"role": "user", "content": "hello world"},
		},
	}
	raw := mustJSON(t, payload)
	r1, err := KvLookup(raw, "chat", "openai|m|k", Options{Tight: true})
	if err != nil {
		t.Fatal(err)
	}
	if r1.Cached != 0 || r1.Fresh != r1.Total || r1.Hit {
		t.Fatalf("first call should be full miss: %+v", r1)
	}
	r2, err := KvLookup(raw, "chat", "openai|m|k", Options{Tight: true})
	if err != nil {
		t.Fatal(err)
	}
	if r2.Cached != r2.Total || r2.Fresh != 0 || !r2.Hit {
		t.Fatalf("repeat should be full hit: %+v", r2)
	}
	if r2.Cached+r2.Fresh != r2.Total {
		t.Fatalf("invariant broken: %+v", r2)
	}
}

func TestKvConversationContinuation(t *testing.T) {
	KvClear("")
	base := map[string]any{
		"messages": []any{
			map[string]any{"role": "user", "content": "first question here"},
		},
	}
	r1, _ := KvLookup(mustJSON(t, base), "chat", "ns1", Options{Tight: true})
	if r1.Cached != 0 {
		t.Fatalf("expected miss, got %+v", r1)
	}
	extended := map[string]any{
		"messages": []any{
			map[string]any{"role": "user", "content": "first question here"},
			map[string]any{"role": "user", "content": "second question here"},
		},
	}
	r2, err := KvLookup(mustJSON(t, extended), "chat", "ns1", Options{Tight: true})
	if err != nil {
		t.Fatal(err)
	}
	if r2.PrefixBlocks != 1 || r2.TotalBlocks != 2 {
		t.Fatalf("expected 1/2 prefix blocks, got %+v", r2)
	}
	if r2.Cached <= 0 || r2.Fresh <= 0 {
		t.Fatalf("expected partial hit, got %+v", r2)
	}
	if r2.Cached+r2.Fresh != r2.Total {
		t.Fatalf("invariant broken: %+v", r2)
	}
}

func TestKvBranching(t *testing.T) {
	KvClear("")
	mk := func(second string) []byte {
		return mustJSON(t, map[string]any{
			"system": "sys",
			"messages": []any{
				map[string]any{"role": "user", "content": "hello"},
				map[string]any{"role": "user", "content": second},
			},
		})
	}
	if _, err := KvLookup(mk("branch A text"), "chat", "nsB", Options{Tight: true}); err != nil {
		t.Fatal(err)
	}
	rB, err := KvLookup(mk("branch B text"), "chat", "nsB", Options{Tight: true})
	if err != nil {
		t.Fatal(err)
	}
	// sys + hello hit, branch B fresh.
	if rB.PrefixBlocks != 2 || rB.TotalBlocks != 3 {
		t.Fatalf("expected 2/3 prefix blocks, got %+v", rB)
	}
	// Re-query A: must still hit fully (branch preserved).
	rA, err := KvLookup(mk("branch A text"), "chat", "nsB", Options{Tight: true})
	if err != nil {
		t.Fatal(err)
	}
	if rA.Fresh != 0 {
		t.Fatalf("branch A should still be cached: %+v", rA)
	}
	st := KvStatsSnapshot()
	if st.Branches < 2 {
		t.Fatalf("expected >=2 branches, got %+v", st)
	}
}

func TestKvNamespaceIsolation(t *testing.T) {
	KvClear("")
	payload := mustJSON(t, map[string]any{
		"messages": []any{map[string]any{"role": "user", "content": "same text"}},
	})
	if _, err := KvLookup(payload, "chat", "nsA", Options{Tight: true}); err != nil {
		t.Fatal(err)
	}
	r, err := KvLookup(payload, "chat", "nsB", Options{Tight: true})
	if err != nil {
		t.Fatal(err)
	}
	if r.Hit {
		t.Fatalf("different namespace must miss: %+v", r)
	}
}

func TestKvProtocolSeparation(t *testing.T) {
	KvClear("")
	payload := mustJSON(t, map[string]any{
		"messages": []any{map[string]any{"role": "user", "content": "same text"}},
	})
	if _, err := KvLookup(payload, "chat", "ns", Options{Tight: true}); err != nil {
		t.Fatal(err)
	}
	// Default: protocol is part of the key - same namespace, different protocol = miss.
	r, err := KvLookup(payload, "anthropic", "ns", Options{Tight: true})
	if err != nil {
		t.Fatal(err)
	}
	if r.Hit {
		t.Fatalf("different protocol must miss by default: %+v", r)
	}
	// Same protocol = hit.
	r2, err := KvLookup(payload, "chat", "ns", Options{Tight: true})
	if err != nil {
		t.Fatal(err)
	}
	if !r2.Hit {
		t.Fatalf("same protocol must hit: %+v", r2)
	}
	// Disabled: keying by namespace only - responses lands on the plain "ns"
	// key while chat already wrote "chat\x00ns". The point is only to
	// assert no protocol-prefixed key is created.
	KvSetSeparateProtocol(false)
	defer KvSetSeparateProtocol(true)
	if _, err := KvLookup(payload, "responses", "ns", Options{Tight: true}); err != nil {
		t.Fatal(err)
	}
	st := KvStatsSnapshot()
	for key := range kvStore {
		if key == "responses\x00ns" {
			t.Fatalf("separation off must not create protocol key, got %+v", st)
		}
	}
}

func TestKvInit(t *testing.T) {
	KvInit(60, 10)
	cfg := KvConfigSnapshot()
	if cfg.TTLSeconds != 60 || cfg.MaxMB != 10 {
		t.Fatalf("bad config: %+v", cfg)
	}
	KvInit(0, 0) // back to defaults
	cfg = KvConfigSnapshot()
	if cfg.TTLSeconds != 600 || cfg.MaxMB != 400 {
		t.Fatalf("bad default: %+v", cfg)
	}
}

func TestKvTTLExpiry(t *testing.T) {
	KvInit(1, 400) // TTL 1s
	defer KvInit(0, 0)
	payload := mustJSON(t, map[string]any{
		"messages": []any{map[string]any{"role": "user", "content": "ttl probe"}},
	})
	if _, err := KvLookup(payload, "chat", "ttl", Options{Tight: true}); err != nil {
		t.Fatal(err)
	}
	r, err := KvLookup(payload, "chat", "ttl", Options{Tight: true})
	if err != nil {
		t.Fatal(err)
	}
	if r.Fresh != 0 {
		t.Fatalf("immediate repeat must hit: %+v", r)
	}
	// Simulate an idle-beyond-TTL namespace without actually sleeping.
	kvMu.Lock()
	kvStore[kvKey("chat", "ttl")].lastAccess -= 2000
	kvMu.Unlock()
	r2, err := KvLookup(payload, "chat", "ttl", Options{Tight: true})
	if err != nil {
		t.Fatal(err)
	}
	if r2.Cached != 0 || r2.Fresh != r2.Total {
		t.Fatalf("expired namespace must full-miss: %+v", r2)
	}
}

func TestKvAnthropicAndResponses(t *testing.T) {
	KvClear("")
	a := mustJSON(t, map[string]any{
		"system":   "sys",
		"messages": []any{map[string]any{"role": "user", "content": "hi there"}},
	})
	if _, err := KvLookup(a, "anthropic", "a", Options{Tight: true}); err != nil {
		t.Fatal(err)
	}
	r, err := KvLookup(a, "anthropic", "a", Options{Tight: true})
	if err != nil {
		t.Fatal(err)
	}
	if r.Fresh != 0 {
		t.Fatalf("anthropic repeat should hit: %+v", r)
	}
	rp := mustJSON(t, map[string]any{
		"instructions": "be nice",
		"input": []any{
			map[string]any{"role": "user", "content": []any{map[string]any{"type": "input_text", "text": "hello"}}},
		},
		"tools": []any{map[string]any{"type": "function", "name": "f", "description": "d"}},
	})
	if _, err := KvLookup(rp, "responses", "r", Options{Tight: true}); err != nil {
		t.Fatal(err)
	}
	r2, err := KvLookup(rp, "responses", "r", Options{Tight: true})
	if err != nil {
		t.Fatal(err)
	}
	if r2.Fresh != 0 {
		t.Fatalf("responses repeat should hit: %+v", r2)
	}
}

func TestKvUnknownProtocol(t *testing.T) {
	if _, err := KvLookup([]byte(`{}`), "nope", "x", Options{}); err == nil {
		t.Fatal("expected error")
	}
}

func TestKvBlockTokenSumMatchesBreakdown(t *testing.T) {
	payload := map[string]any{
		"system": "You are helpful.",
		"messages": []any{
			map[string]any{"role": "user", "content": []any{
				map[string]any{"type": "text", "text": "hello"},
				map[string]any{"type": "text", "text": "world"},
			}},
			map[string]any{"role": "assistant", "content": "ok"},
		},
		"tools": []any{
			map[string]any{"type": "function", "function": map[string]any{"name": "f", "description": "d"}},
		},
	}
	raw := mustJSON(t, payload)
	blocks, bd, err := kvLinearize(raw, "chat", Options{Tight: true})
	if err != nil {
		t.Fatal(err)
	}
	sum := 0
	for _, b := range blocks {
		sum += b.Tokens
	}
	if sum != bd.System+bd.Messages+bd.Tools {
		t.Fatalf("block sum %d != breakdown %d (bd=%+v)", sum, bd.System+bd.Messages+bd.Tools, bd)
	}
}
