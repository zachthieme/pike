package scanner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"pike/internal/model"
)

func writeBenchFiles(b *testing.B, dir string, count int) {
	b.Helper()
	for i := range count {
		name := filepath.Join(dir, fmt.Sprintf("note_%04d.md", i))
		var content string
		for j := range 50 {
			switch j % 5 {
			case 0:
				content += fmt.Sprintf("- [ ] task %d-%d @due(2026-03-16)\n", i, j)
			case 1:
				content += fmt.Sprintf("- [x] done %d-%d @completed(2026-03-10)\n", i, j)
			case 2:
				content += fmt.Sprintf("- bullet %d-%d @today\n", i, j)
			default:
				content += fmt.Sprintf("This is just regular text line %d-%d\n", i, j)
			}
		}
		if err := os.WriteFile(name, []byte(content), 0o644); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkScan(b *testing.B) {
	for _, count := range []int{100, 500, 1000} {
		b.Run(fmt.Sprintf("files=%d", count), func(b *testing.B) {
			dir := b.TempDir()
			writeBenchFiles(b, dir, count)
			sc, err := New(dir, []string{"**/*.md"}, nil)
			if err != nil {
				b.Fatal(err)
			}
			ctx := context.Background()
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				sc.mtimes = make(map[string]time.Time)
				sc.tasks = make(map[string][]model.Task)
				if _, err := sc.Scan(ctx); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkRefreshNoChanges(b *testing.B) {
	for _, count := range []int{100, 500, 1000} {
		b.Run(fmt.Sprintf("files=%d", count), func(b *testing.B) {
			dir := b.TempDir()
			writeBenchFiles(b, dir, count)
			sc, err := New(dir, []string{"**/*.md"}, nil)
			if err != nil {
				b.Fatal(err)
			}
			ctx := context.Background()
			if _, err := sc.Scan(ctx); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				if _, err := sc.Refresh(ctx); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
