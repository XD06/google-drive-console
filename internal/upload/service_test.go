package upload

import (
	"context"
	"errors"
	"testing"

	"github.com/dsk/drive-backup-console/internal/drive"
)

type mockDrive struct {
	session string
	ranges  []struct{ start, end, total int64 }
	failAt  int
	calls   int
}

func (m *mockDrive) ResumableStart(ctx context.Context, name, mimeType, parentID string, size int64) (string, error) {
	if m.session == "" {
		m.session = "http://session.test/1"
	}
	return m.session, nil
}

func (m *mockDrive) UploadRange(ctx context.Context, sessionURL string, start, end, total int64, data []byte) (drive.UploadChunkResult, error) {
	m.calls++
	m.ranges = append(m.ranges, struct{ start, end, total int64 }{start, end, total})
	if m.failAt > 0 && m.calls == m.failAt {
		return drive.UploadChunkResult{}, errors.New("drive fail")
	}
	next := end + 1
	if next >= total {
		return drive.UploadChunkResult{Complete: true, FileID: "fid1", NextByte: total}, nil
	}
	return drive.UploadChunkResult{Complete: false, NextByte: next}, nil
}

func (m *mockDrive) UploadEmpty(ctx context.Context, sessionURL string) (drive.UploadChunkResult, error) {
	return drive.UploadChunkResult{Complete: true, FileID: "empty1", NextByte: 0}, nil
}
func (m *mockDrive) QueryUploadStatus(ctx context.Context, sessionURL string, total int64) (drive.UploadChunkResult, error) {
	return drive.UploadChunkResult{Complete: true, FileID: "fid1", NextByte: total}, nil
}

func TestCreateAndChunkAligned(t *testing.T) {
	md := &mockDrive{}
	svc := &Service{Store: NewStore(), Drive: md}
	// 256KiB + 10 bytes
	total := int64(drive.ChunkMultiple + 10)
	j, err := svc.Create(context.Background(), CreateInput{Name: "a.bin", Size: total})
	if err != nil {
		t.Fatal(err)
	}
	if j.Status != StatusPending {
		t.Fatalf("status %s", j.Status)
	}

	// First client chunk: full 256KiB.
	// With DefaultFlushSize=16MiB, this is buffered but not flushed yet.
	chunk1 := make([]byte, drive.ChunkMultiple)
	j, err = svc.AppendChunk(context.Background(), j.ID, 0, chunk1)
	if err != nil {
		t.Fatal(err)
	}
	// BytesSent may be 0 because the chunk is buffered (below flush threshold).
	// Status should be Uploading.
	if j.Status != StatusUploading {
		t.Fatalf("status %s", j.Status)
	}

	// Final 10 bytes — triggers flush of all buffered data.
	j, err = svc.AppendChunk(context.Background(), j.ID, drive.ChunkMultiple, make([]byte, 10))
	if err != nil {
		t.Fatal(err)
	}
	if j.Status != StatusCompleted || j.FileID != "fid1" {
		t.Fatalf("%+v", j.View())
	}
	if len(md.ranges) < 1 {
		t.Fatalf("drive puts %d", len(md.ranges))
	}
}

func TestCreateZero(t *testing.T) {
	svc := &Service{Store: NewStore(), Drive: &mockDrive{}}
	j, err := svc.Create(context.Background(), CreateInput{Name: "empty", Size: 0})
	if err != nil {
		t.Fatal(err)
	}
	if j.Status != StatusCompleted || j.FileID != "empty1" {
		t.Fatalf("%+v", j)
	}
}

func TestOffsetMismatch(t *testing.T) {
	svc := &Service{Store: NewStore(), Drive: &mockDrive{}}
	j, err := svc.Create(context.Background(), CreateInput{Name: "a", Size: 100})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.AppendChunk(context.Background(), j.ID, 5, []byte("hi"))
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("want validation, got %v", err)
	}
}

func TestSmallFileSingleChunk(t *testing.T) {
	md := &mockDrive{}
	svc := &Service{Store: NewStore(), Drive: md}
	j, err := svc.Create(context.Background(), CreateInput{Name: "s.txt", Size: 5, MimeType: "text/plain"})
	if err != nil {
		t.Fatal(err)
	}
	j, err = svc.AppendChunk(context.Background(), j.ID, -1, []byte("hello"))
	if err != nil {
		t.Fatal(err)
	}
	if j.Status != StatusCompleted {
		t.Fatalf("%s", j.Status)
	}
}