package codec

import (
	"sync"
	"unicode"
	"unicode/utf8"
)

type vocab map[string]uint

var asciiLetter [256]bool
var asciiNumber [256]bool
var asciiSpace [256]bool
var asciiB1Pre [256]bool
var asciiB3Sym [256]bool

func init() {
	for c := 0; c < 256; c++ {
		r := rune(c)
		asciiLetter[c] = r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z'
		asciiNumber[c] = r >= '0' && r <= '9'
		asciiSpace[c] = c == ' ' || c == '\t' || c == '\n' || c == '\v' || c == '\f' || c == '\r'
		asciiB1Pre[c] = !asciiLetter[c] && !asciiNumber[c] && r != '\n' && r != '\r' && r != 0x7F
		asciiB3Sym[c] = !asciiSpace[c] && !asciiLetter[c] && !asciiNumber[c] && r != 0x7F
	}
}

const uniMax = 0x2500

func uniBit(tab *[148]uint64, r rune) bool {
	u := uint(r)
	if u >= uniMax {
		return false
	}
	return tab[u/64]&(1<<(u%64)) != 0
}

func isL(r rune) bool {
	if r < 128 {
		return asciiLetter[r]
	}
	if r < uniMax {
		return uniBit(&uniL, r)
	}
	return unicode.Is(unicode.L, r)
}

func isN(r rune) bool {
	if r < 128 {
		return asciiNumber[r]
	}
	if r < uniMax {
		return uniBit(&uniN, r)
	}
	return unicode.Is(unicode.N, r)
}

func isS(r rune) bool {
	if r < 128 {
		return asciiSpace[r]
	}
	if r < uniMax {
		return uniBit(&uniS, r)
	}
	return unicode.IsSpace(r)
}

func isNL(r rune) bool { return r == '\n' || r == '\r' }

func isB1Pre(r rune) bool {
	if r < 128 {
		return asciiB1Pre[r]
	}
	if r == '\n' || r == '\r' {
		return false
	}
	if r < uniMax {
		return !uniBit(&uniL, r) && !uniBit(&uniN, r)
	}
	return !unicode.Is(unicode.L, r) && !unicode.Is(unicode.N, r)
}

func isB3Sym(r rune) bool {
	if r < 128 {
		return asciiB3Sym[r]
	}
	if r == 0x7F {
		return false
	}
	if r < uniMax {
		return !uniBit(&uniS, r) && !uniBit(&uniL, r) && !uniBit(&uniN, r)
	}
	return !unicode.IsSpace(r) && !unicode.Is(unicode.L, r) && !unicode.Is(unicode.N, r)
}

func decodeAt(s string, pos int) (rune, int) {
	c := s[pos]
	if c < 0x80 {
		return rune(c), 1
	}
	r, size := utf8.DecodeRuneInString(s[pos:])
	if r == utf8.RuneError && size <= 1 {
		return utf8.RuneError, 1
	}
	return r, size
}

func matchContraAt(s string, pos int) int {
	if s[pos] != '\'' {
		return 0
	}
	if pos+1 >= len(s) {
		return 0
	}
	c := s[pos+1]
	switch {
	case c == 's' || c == 'S' || c == 't' || c == 'T' || c == 'm' || c == 'M' || c == 'd' || c == 'D':
		return 2
	case c == 'r' || c == 'R' || c == 'v' || c == 'V':
		if pos+2 < len(s) && (s[pos+2] == 'e' || s[pos+2] == 'E') {
			return 3
		}
		return 0
	case c == 'l' || c == 'L':
		if pos+2 < len(s) && (s[pos+2] == 'l' || s[pos+2] == 'L') {
			return 3
		}
		return 0
	default:
		return 0
	}
}

func scanLetters(s string, n, i int) int {
	for i < n {
		if s[i] < 0x80 {
			if !asciiLetter[s[i]] {
				break
			}
			i++
			continue
		}
		r, size := decodeAt(s, i)
		if !isL(r) {
			break
		}
		i += size
	}
	return i
}

func splitASCII(s string, pos int) int {
	n := len(s)
	c := s[pos]
	if asciiLetter[c] {
		i := pos + 1
		for i < n && asciiLetter[s[i]] {
			i++
		}
		if i < n && s[i] >= 0x80 {
			return scanLetters(s, n, i) - pos
		}
		return i - pos
	}
	if c >= '0' && c <= '9' {
		m := pos + 1
		if m < n && s[m] >= '0' && s[m] <= '9' {
			m++
			if m < n && s[m] >= '0' && s[m] <= '9' {
				m++
			}
		}
		return m - pos
	}
	if asciiB1Pre[c] && pos+1 < n {
		if s[pos+1] < 0x80 {
			if asciiLetter[s[pos+1]] {
				i := pos + 2
				for i < n && asciiLetter[s[i]] {
					i++
				}
				if i < n && s[i] >= 0x80 {
					return scanLetters(s, n, i) - pos
				}
				return i - pos
			}
		} else if r, _ := decodeAt(s, pos+1); isL(r) {
			return scanLetters(s, n, pos+1) - pos
		}
	}
	j := pos
	if c == ' ' {
		j = pos + 1
	}
	k := j
	if k < n {
		if s[k] < 0x80 {
			if asciiB3Sym[s[k]] {
				for k < n && s[k] < 0x80 && asciiB3Sym[s[k]] {
					k++
				}
				if k < n && s[k] >= 0x80 {
					if r, size := decodeAt(s, k); isB3Sym(r) {
						k = scanSyms(s, n, k+size)
					}
				}
			}
		} else if r, size := decodeAt(s, k); isB3Sym(r) {
			k = scanSyms(s, n, k+size)
		}
	}
	if k > j {
		for k < n && (s[k] == '\n' || s[k] == '\r') {
			k++
		}
		return k - pos
	}
	return splitSpaces(s, n, pos)
}

func scanSyms(s string, n, k int) int {
	for k < n {
		if s[k] < 0x80 {
			if !asciiB3Sym[s[k]] {
				break
			}
			k++
			continue
		}
		r, size := decodeAt(s, k)
		if !isB3Sym(r) {
			break
		}
		k += size
	}
	return k
}

func splitSpaces(s string, n, pos int) int {
	e := pos
	for e < n {
		if s[e] < 0x80 {
			if !asciiSpace[s[e]] {
				break
			}
			e++
			continue
		}
		r, size := decodeAt(s, e)
		if !isS(r) {
			break
		}
		e += size
	}
	if e == pos {
		return 0
	}
	for q := pos; q < e; {
		var r rune
		var size int
		if s[q] < 0x80 {
			r, size = rune(s[q]), 1
		} else {
			r, size = decodeAt(s, q)
		}
		if isNL(r) {
			f := q
			for f < n && (s[f] == '\n' || s[f] == '\r') {
				f++
			}
			return f - pos
		}
		q += size
	}
	if e < n {
		if e-pos == 1 {
			return 1
		}
		back := e - 1
		for back > pos && s[back] >= 0x80 && s[back] < 0xC0 {
			back--
		}
		if back == pos {
			return e - pos
		}
		return back - pos
	}
	return e - pos
}

func splitRune(s string, pos int, r0 rune, size0 int) int {
	n := len(s)
	if isB1Pre(r0) {
		if i := scanLetters(s, n, pos+size0); i > pos+size0 {
			return i - pos
		}
	}
	if isL(r0) {
		return scanLetters(s, n, pos+size0) - pos
	}
	m := pos
	sz := size0
	for m < n && m-pos < 3+sz {
		var r rune
		var size int
		if s[m] < 0x80 {
			r, size = rune(s[m]), 1
		} else {
			r, size = decodeAt(s, m)
		}
		if !isN(r) {
			break
		}
		m += size
		if m-pos >= 3 && s[m-1] < 0x80 {
			break
		}
	}
	if m > pos {
		cnt := 0
		for q := pos; q < m; {
			_, size := decodeAt(s, q)
			q += size
			cnt++
			if cnt >= 3 {
				break
			}
		}
		if cnt > 3 {
			cnt = 3
		}
		end := pos
		for i := 0; i < cnt; i++ {
			_, size := decodeAt(s, end)
			end += size
		}
		return end - pos
	}
	j := pos
	if r0 == ' ' {
		j = pos + size0
	}
	k := scanSyms(s, n, j)
	if k > j {
		if k == j {
			return 0
		}
		for k < n && (s[k] == '\n' || s[k] == '\r') {
			k++
		}
		return k - pos
	}
	return splitSpaces(s, n, pos)
}

type shardCache struct {
	mu    sync.RWMutex
	cache map[string]int
}

type fastCodec struct {
	rankOf map[string]uint32
	shards [16]shardCache
}

var (
	fastOnce sync.Once
	fast     *fastCodec
)

func getFast() *fastCodec {
	fastOnce.Do(func() {
		BaseVocabOnce.Do(BaseVocabInit)
		r := make(map[string]uint32, len(BaseVocab)+1)
		for k, v := range BaseVocab {
			r[k] = uint32(v)
		}
		fast = &fastCodec{rankOf: r}
		for i := range fast.shards {
			fast.shards[i].cache = make(map[string]int)
		}
	})
	return fast
}

const maxRank = uint32(0xFFFFFFFF)

func (f *fastCodec) countPiece(piece string) int {
	if len(piece) <= 1 {
		return len(piece)
	}
	if _, ok := f.rankOf[piece]; ok {
		return 1
	}
	h := uint32(2166136261)
	for i := 0; i < len(piece); i++ {
		h = (h ^ uint32(piece[i])) * 16777619
	}
	s := &f.shards[h&15]
	s.mu.RLock()
	v, ok := s.cache[piece]
	s.mu.RUnlock()
	if ok {
		return v
	}
	n := f.countBPE(piece)
	s.mu.Lock()
	if len(s.cache) >= 4096 {
		s.cache = make(map[string]int, 4096)
	}
	s.cache[piece] = n
	s.mu.Unlock()
	return n
}

func (f *fastCodec) countBPEScan(piece string) int {
	n := len(piece)
	var offBuf [128]int
	var rankBuf [128]uint32
	var offsets []int
	var ranks []uint32
	if n+1 <= len(offBuf) {
		offsets = offBuf[:n+1]
		for i := 0; i <= n; i++ {
			offsets[i] = i
		}
	} else {
		offsets = make([]int, n+1)
		for i := 0; i <= n; i++ {
			offsets[i] = i
		}
	}
	m := n
	if m <= len(rankBuf) {
		ranks = rankBuf[:m]
	} else {
		ranks = make([]uint32, m)
	}
	for i := 0; i < m; i++ {
		if i+2 <= n {
			ranks[i] = f.pairRank(piece, offsets[i], offsets[i+2])
		} else {
			ranks[i] = maxRank
		}
	}
	count := m
	for m > 1 {
		best := maxRank
		bi := -1
		for i := 0; i < m; i++ {
			if ranks[i] < best {
				best = ranks[i]
				bi = i
			}
		}
		if bi < 0 {
			break
		}
		copy(offsets[bi+1:], offsets[bi+2:])
		offsets = offsets[:len(offsets)-1]
		m--
		count--
		if m <= 1 {
			break
		}
		if bi < m {
			if bi+2 < len(offsets) {
				ranks[bi] = f.pairRank(piece, offsets[bi], offsets[bi+2])
			} else {
				ranks[bi] = maxRank
			}
		}
		if bi > 0 {
			if bi+1 < len(offsets) {
				ranks[bi-1] = f.pairRank(piece, offsets[bi-1], offsets[bi+1])
			} else {
				ranks[bi-1] = maxRank
			}
		}
		copy(ranks[bi+1:], ranks[bi+2:])
		ranks = ranks[:m]
	}
	return count
}

type heapItem struct {
	rank uint32
	idx  int
}

func (f *fastCodec) countBPE(piece string) int {
	n := len(piece)
	if n <= 1 {
		return n
	}
	if n <= 24 {
		return f.countBPEScan(piece)
	}
	nxt := make([]int, n+1)
	prv := make([]int, n+1)
	alive := make([]bool, n+1)
	for i := 0; i <= n; i++ {
		nxt[i] = i + 1
		prv[i] = i - 1
		alive[i] = true
	}
	nxt[n] = -1
	h := make([]heapItem, 0, n)
	for i := 0; i+2 <= n; i++ {
		if r := f.pairRank(piece, i, i+2); r != maxRank {
			h = pushHeap(h, heapItem{r, i})
		}
	}
	count := n
	for len(h) > 0 {
		top := h[0]
		h = popHeap(h)
		i := top.idx
		if !alive[i] {
			continue
		}
		j := nxt[i]
		if j < 0 || j > n || !alive[j] {
			continue
		}
		k := nxt[j]
		if k < 0 || k > n || !alive[k] {
			continue
		}
		r := f.pairRank(piece, i, k)
		if r != top.rank {
			if r != maxRank {
				h = pushHeap(h, heapItem{r, i})
			}
			continue
		}
		alive[j] = false
		nxt[i] = k
		prv[k] = i
		count--
		if kk := nxt[k]; kk >= 0 && kk <= n && alive[kk] {
			if nn := nxt[kk]; nn >= 0 && nn <= n {
				if r2 := f.pairRank(piece, k, nn); r2 != maxRank {
					h = pushHeap(h, heapItem{r2, k})
				}
			}
		}
		if kk := nxt[i]; kk >= 0 && kk <= n && alive[kk] {
			if nn := nxt[kk]; nn >= 0 && nn <= n {
				if r2 := f.pairRank(piece, i, nn); r2 != maxRank {
					h = pushHeap(h, heapItem{r2, i})
				}
			}
		}
		if p := prv[i]; p >= 0 && alive[p] {
			if r2 := f.pairRank(piece, p, k); r2 != maxRank {
				h = pushHeap(h, heapItem{r2, p})
			}
		}
	}
	return count
}

func pushHeap(h []heapItem, it heapItem) []heapItem {
	h = append(h, it)
	i := len(h) - 1
	for i > 0 {
		p := (i - 1) / 2
		if h[p].rank < h[i].rank || (h[p].rank == h[i].rank && h[p].idx <= h[i].idx) {
			break
		}
		h[p], h[i] = h[i], h[p]
		i = p
	}
	return h
}

func popHeap(h []heapItem) []heapItem {
	n := len(h) - 1
	h[0] = h[n]
	h = h[:n]
	i := 0
	for {
		l := 2*i + 1
		r := l + 1
		m := i
		if l < len(h) && (h[l].rank < h[m].rank || (h[l].rank == h[m].rank && h[l].idx < h[m].idx)) {
			m = l
		}
		if r < len(h) && (h[r].rank < h[m].rank || (h[r].rank == h[m].rank && h[r].idx < h[m].idx)) {
			m = r
		}
		if m == i {
			break
		}
		h[m], h[i] = h[i], h[m]
		i = m
	}
	return h
}

func (f *fastCodec) pairRank(piece string, a, d int) uint32 {
	if a < 0 || d > len(piece) || a >= d {
		return maxRank
	}
	if r, ok := f.rankOf[piece[a:d]]; ok {
		return r
	}
	return maxRank
}

func FastCountNoErr(input string) (int, error) {
	return FastCount(input), nil
}

const textCacheMax = 4096

type textCacheShard struct {
	mu sync.Mutex
	m  map[string]int
}

var textCacheShards [16]textCacheShard

func textCacheLoad(s string) (int, bool) {
	h := uint32(2166136261)
	for i := 0; i < len(s); i++ {
		h = (h ^ uint32(s[i])) * 16777619
	}
	sh := &textCacheShards[h&15]
	sh.mu.Lock()
	defer sh.mu.Unlock()
	if sh.m == nil {
		return 0, false
	}
	v, ok := sh.m[s]
	return v, ok
}

func textCacheStore(s string, n int) {
	h := uint32(2166136261)
	for i := 0; i < len(s); i++ {
		h = (h ^ uint32(s[i])) * 16777619
	}
	sh := &textCacheShards[h&15]
	sh.mu.Lock()
	defer sh.mu.Unlock()
	if sh.m == nil {
		sh.m = make(map[string]int, 256)
	}
	if len(sh.m) >= textCacheMax/16 {
		sh.m = make(map[string]int, 256)
	}
	sh.m[s] = n
}

func FastCount(input string) int {
	if len(input) >= 64 {
		if v, ok := textCacheLoad(input); ok {
			return v
		}
	}
	n := getFast().countText(input)
	if len(input) >= 64 {
		textCacheStore(input, n)
	}
	return n
}

func (f *fastCodec) countText(input string) int {
	pos := 0
	n := len(input)
	total := 0
	for pos < n {
		c := input[pos]
		if c == 0x7F {
			pos++
			continue
		}
		if c < 0x80 {
			l := splitASCII(input, pos)
			if l <= 0 {
				pos++
				continue
			}
			end := pos + l
			if end > n {
				end = n
			}
			total += f.countPiece(input[pos:end])
			pos = end
			continue
		}
		r0, size0 := decodeAt(input, pos)
		l := splitRune(input, pos, r0, size0)
		if l <= 0 {
			pos += size0
			continue
		}
		end := pos + l
		if end > n {
			end = n
		}
		total += f.countPiece(input[pos:end])
		pos = end
	}
	return total
}
