package upload

import (
	"context"
	"testing"

	"github.com/dsk/drive-backup-console/internal/drive"
)

// ---------------------------------------------------------------------------
// Benchmark: AppendChunk throughput (bytes/sec through flush loop to Drive)
// ---------------------------------------------------------------------------

// benchDrive counts UploadRange calls.
type benchDrive struct {
	calls int
}

func (m *benchDrive) ResumableStart(ctx context.Context, name, mimeType, parentID string, size int64) (string, error) {
	return "http://bench.test/session/1", nil
}

func (m *benchDrive) UploadRange(ctx context.Context, sessionURL string, start, end, total int64, data []byte) (drive.UploadChunkResult, error) {
	m.calls++
	next := end + 1
	if next >= total {
		return drive.UploadChunkResult{Complete: true, FileID: "bench_fid", NextByte: total}, nil
	}
	return drive.UploadChunkResult{Complete: false, NextByte: next}, nil
}

func (m *benchDrive) UploadEmpty(ctx context.Context, sessionURL string) (drive.UploadChunkResult, error) {
	return drive.UploadChunkResult{Complete: true, FileID: "empty_bench", NextByte: 0}, nil
}

func (m *benchDrive) QueryUploadStatus(ctx context.Context, sessionURL string, total int64) (drive.UploadChunkResult, error) {
	return drive.UploadChunkResult{Complete: true, FileID: "bench_fid", NextByte: total}, nil
}

// BenchmarkAppendChunk_8MB_File sends one 8MB file in 8MB client chunk.
// This measures the flush loop throughput: how fast the server re-chunks
// 8MB of client data into 256KB (current) Drive chunks.
func BenchmarkAppendChunk_8MB_File(b *testing.B) {
	benchmarkAppendChunk(b, 8<<20, 8<<20)
}

// BenchmarkAppendChunk_32MB_File sends a 32MB file in 8MB client chunks.
func BenchmarkAppendChunk_32MB_File(b *testing.B) {
	benchmarkAppendChunk(b, 32<<20, 8<<20)
}

// BenchmarkAppendChunk_100MB_File sends a 100MB file in 8MB client chunks.
func BenchmarkAppendChunk_100MB_File(b *testing.B) {
	benchmarkAppendChunk(b, 100<<20, 8<<20)
}

func benchmarkAppendChunk(b *testing.B, totalSize int64, clientChunk int) {
	b.SetBytes(totalSize)
	b.ReportAllocs()

	chunk := make([]byte, clientChunk)
	ctx := context.Background()

	for i := 0; i < b.N; i++ {
		bd := &benchDrive{}
		svc := &Service{Store: NewStore(), Drive: bd}
		j, err := svc.Create(ctx, CreateInput{
			Name: "bench.bin",
			Size: totalSize,
		})
		if err != nil {
			b.Fatal(err)
		}

		var offset int64
		for offset < totalSize {
			end := offset + int64(clientChunk)
			if end > totalSize {
				end = totalSize
			}
			chunkData := chunk[:end-offset]
			j, err = svc.AppendChunk(ctx, j.ID, offset, chunkData)
			if err != nil {
				b.Fatal(err)
			}
			offset = j.BytesReceived
		}

		if j.Status != StatusCompleted {
			b.Fatalf("expected completed, got %s (err: %s)", j.Status, j.Error)
		}
		// Report how many Drive API calls were made (important metric)
		if i == 0 {
			b.Logf("Drive API calls for %dMB file: %d", totalSize>>20, bd.calls)
		}
	}
}
