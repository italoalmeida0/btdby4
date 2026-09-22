package btdby4_test

import (
	"os"
	"testing"

	"github.com/italoalmeida0/btdby4"
)

var kvPerfFiles = []string{
	"context_20k.json",
	"context_50k.json",
	"context_100k.json",
	"context_200k.json",
	"context_full.json",
	"context_rich_10k.json",
}

func loadRaw(b *testing.B, f string) []byte {
	b.Helper()
	raw, err := os.ReadFile(f)
	if err != nil {
		b.Skipf("missing %s", f)
	}
	return raw
}

// Baseline: plain counting, no KV.
func BenchmarkKvPerfCount(b *testing.B) {
	for _, f := range kvPerfFiles {
		raw := loadRaw(b, f)
		b.Run(f, func(b *testing.B) {
			b.SetBytes(int64(len(raw)))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := btdby4.CountChatRequestJSON(raw, btdby4.Options{Tight: true}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// Cold miss: clears the namespace every iteration (parse + fused count/linearize + commit).
func BenchmarkKvPerfMiss(b *testing.B) {
	for _, f := range kvPerfFiles {
		raw := loadRaw(b, f)
		b.Run(f, func(b *testing.B) {
			b.SetBytes(int64(len(raw)))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				btdby4.KvClear("perf")
				if _, err := btdby4.KvLookup(raw, "chat", "perf", btdby4.Options{Tight: true}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// Warm hit: same payload repeated (parse + fused count/linearize + walk, no commit).
func TestKvPerfCorrectness(t *testing.T) {
	for _, f := range kvPerfFiles {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Skipf("missing %s", f)
		}
		ns := "correctness:" + f
		btdby4.KvClear(ns)
		r1, err := btdby4.KvLookup(raw, "chat", ns, btdby4.Options{Tight: true})
		if err != nil {
			t.Fatalf("%s miss: %v", f, err)
		}
		r2, err := btdby4.KvLookup(raw, "chat", ns, btdby4.Options{Tight: true})
		if err != nil {
			t.Fatalf("%s hit: %v", f, err)
		}
		t.Logf("%s: total=%d blocks=%d | miss(cached=%d fresh=%d) hit(cached=%d fresh=%d ratio=%.4f)",
			f, r1.Total, r1.TotalBlocks, r1.Cached, r1.Fresh, r2.Cached, r2.Fresh, r2.HitRatio)
		if r1.Cached != 0 || r1.Fresh != r1.Total {
			t.Fatalf("%s: first call must be full miss: %+v", f, r1)
		}
		if r2.Fresh != 0 || r2.Cached != r2.Total {
			t.Fatalf("%s: repeat must be full hit: %+v", f, r2)
		}
	}
}

func BenchmarkKvPerfHit(b *testing.B) {
	for _, f := range kvPerfFiles {
		raw := loadRaw(b, f)
		b.Run(f, func(b *testing.B) {
			b.SetBytes(int64(len(raw)))
			btdby4.KvClear("perf")
			if _, err := btdby4.KvLookup(raw, "chat", "perf", btdby4.Options{Tight: true}); err != nil {
				b.Fatal(err)
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := btdby4.KvLookup(raw, "chat", "perf", btdby4.Options{Tight: true}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
