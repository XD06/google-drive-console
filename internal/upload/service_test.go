package upload

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
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

// Regression: AppendChunk must persist job state transitions (uploading →
// completed) to the PersistentStore, so a crash mid-upload no longer makes the
// job vanish from uploads.json.
func TestAppendChunkPersistsStates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "uploads.json")
	ps := NewPersistentStore(path)
	md := &mockDrive{}
	svc := &Service{Store: ps, Drive: md}

	total := int64(10 * drive.ChunkMultiple) // 2.5 MiB
	j, err := svc.Create(context.Background(), CreateInput{Name: "p.bin", Size: total})
	if err != nil {
		t.Fatal(err)
	}

	// Partial append: 1 MiB sits in the buffer (below the 16 MiB flush
	// threshold), so no Drive call happens and the job stays uploading.
	part := int64(4 * drive.ChunkMultiple)
	if _, err = svc.AppendChunk(context.Background(), j.ID, 0, make([]byte, part)); err != nil {
		t.Fatal(err)
	}
	ps.SaveNow()

	// The on-disk record must show the uploading state with the received bytes.
	var recs []struct {
		UploadID      string `json:"uploadId"`
		Status        string `json:"status"`
		BytesReceived int64  `json:"bytesReceived"`
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("uploads.json was not written: %v", err)
	}
	if err := json.Unmarshal(raw, &recs); err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 || recs[0].UploadID != j.ID ||
		recs[0].Status != string(StatusUploading) || recs[0].BytesReceived != part {
		t.Fatalf("persisted record = %+v, want id=%s status=%s bytes=%d", recs, j.ID, StatusUploading, part)
	}

	// Reload from the same path: the record survives the restart. Per the
	// restart policy an "uploading" job is marked failed, but the bytes are kept.
	ps2 := NewPersistentStore(path)
	defer ps2.StopReaper()
	j2, err := ps2.Get(j.ID)
	if err != nil {
		t.Fatalf("reloaded store lost in-flight job: %v", err)
	}
	if j2.BytesReceived != part {
		t.Errorf("reloaded BytesReceived = %d, want %d", j2.BytesReceived, part)
	}

	// Finish the upload, then verify completion is persisted too.
	if _, err = svc.AppendChunk(context.Background(), j.ID, part, make([]byte, total-part)); err != nil {
		t.Fatal(err)
	}
	ps.SaveNow()
	j3, err := ps.Get(j.ID)
	if err != nil {
		t.Fatal(err)
	}
	if j3.Status != StatusCompleted || j3.FileID != "fid1" {
		t.Fatalf("after finish: status=%s fileId=%s", j3.Status, j3.FileID)
	}
	raw, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &recs); err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 || recs[0].Status != string(StatusCompleted) || recs[0].UploadID != j.ID {
		t.Fatalf("persisted final record = %+v, want status=%s", recs, StatusCompleted)
	}
}
