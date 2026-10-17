package store

import (
	"fmt"
	"testing"
)

// Setup is deliberately excluded from sub-benchmark timings.
func BenchmarkQueryClasses(b *testing.B) {
	s, err := Open(b.TempDir())
	if err != nil {
		b.Fatal(err)
	}
	defer s.Close()
	for i := 0; i < 10000; i++ {
		content := fmt.Sprintf("Synthetic item %d unique words", i)
		if err := s.SaveItem(&Item{Hash: fmt.Sprint(i), Type: "text", Mime: "text/plain", Content: content, Preview: content, SizeBytes: int64(len(content))}); err != nil {
			b.Fatal(err)
		}
	}
	for _, q := range []string{"Synthetic", "item 9999", "no-such-clip", "it", "type:text unique words"} {
		b.Run(q, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := s.QueryItems(QueryOptions{Query: q, Limit: 50}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
