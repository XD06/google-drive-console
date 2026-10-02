package download

import (
	"testing"
	"time"
)

func TestShouldUseProxy(t *testing.T) {
	proxy := "socks5://127.0.0.1:10808"

	tests := []struct {
		url    string
		want   bool
		reason string
	}{
		{"https://www.youtube.com/watch?v=abc", true, "YouTube needs proxy"},
		{"https://youtu.be/abc", true, "YouTube short link needs proxy"},
		{"https://www.bilibili.com/video/BV1abc", false, "Bilibili should not use proxy"},
		{"https://b23.tv/abc123", false, "Bilibili short link should not use proxy"},
		{"https://www.douyin.com/video/123", false, "Douyin should not use proxy"},
		{"https://v.douyin.com/abc123", false, "Douyin short link should not use proxy"},
		{"https://www.xiaohongshu.com/explore/abc", false, "Xiaohongshu should not use proxy"},
		{"https://xhslink.com/abc", false, "Xiaohongshu short link should not use proxy"},
		{"https://example.cn/page", false, ".cn domain should not use proxy"},
		{"https://example.com/video", true, "Generic .com should use proxy"},
		{"https://example.aliyuncs.com/file", false, "aliyuncs should not use proxy"},
		{"https://example.myqcloud.com/file", false, "myqcloud should not use proxy"},
		{"https://www.tiktok.com/@user/video/123", true, "TikTok international needs proxy"},
		{"https://www.instagram.com/reel/abc", true, "Instagram needs proxy"},
		{"https://x.com/user/status/123", true, "Twitter/X needs proxy"},
		{"", true, "Empty URL defaults to proxy (unknown)"},
	}

	for _, tt := range tests {
		t.Run(tt.reason, func(t *testing.T) {
			got := ShouldUseProxy(tt.url, proxy)
			if got != tt.want {
				t.Errorf("ShouldUseProxy(%q) = %v, want %v (%s)", tt.url, got, tt.want, tt.reason)
			}
		})
	}

	// When proxy is empty, always returns false
	if ShouldUseProxy("https://www.youtube.com/watch?v=abc", "") {
		t.Error("ShouldUseProxy should return false when proxy is empty")
	}
}

func TestNeedsCookies(t *testing.T) {
	tests := []struct {
		url    string
		want   bool
		reason string
	}{
		{"https://www.douyin.com/video/123", true, "Douyin needs cookies"},
		{"https://v.douyin.com/abc123", true, "Douyin short link needs cookies"},
		{"https://www.xiaohongshu.com/explore/abc", true, "Xiaohongshu needs cookies"},
		{"https://xhslink.com/abc", true, "Xiaohongshu short link needs cookies"},
		{"https://www.youtube.com/watch?v=abc", false, "YouTube does not need cookies"},
		{"https://www.bilibili.com/video/BV1abc", true, "Bilibili needs cookies"},
		{"https://www.tiktok.com/@user/video/123", true, "TikTok needs cookies"},
		{"https://example.com/file.mp4", false, "Generic URL does not need cookies"},
	}

	for _, tt := range tests {
		t.Run(tt.reason, func(t *testing.T) {
			got := NeedsCookies(tt.url)
			if got != tt.want {
				t.Errorf("NeedsCookies(%q) = %v, want %v (%s)", tt.url, got, tt.want, tt.reason)
			}
		})
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello world", "hello world"},
		{"file/name.mp4", "file_name.mp4"},
		{"file\\name.mp4", "file_name.mp4"},
		{"file:name.mp4", "file_name.mp4"},
		{"file*name?.mp4", "file_name_.mp4"},
		{`file"name<>.mp4`, "file_name_.mp4"},
		{"file|name.mp4", "file_name.mp4"},
		{"", ""},
		{"   ", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := sanitizeFilename(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeFilename(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestGuessExt(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://example.com/video.mp4", "mp4"},
		{"https://example.com/video.webm", "webm"},
		{"https://example.com/video.mp4?token=abc", "mp4"},
		{"https://example.com/download", "mp4"}, // default
		{"https://example.com/file.mkv", "mkv"},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			got := guessExt(tt.url)
			if got != tt.want {
				t.Errorf("guessExt(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestGuessMimeType(t *testing.T) {
	tests := []struct {
		ext  string
		want string
	}{
		{"mp4", "video/mp4"},
		{"webm", "video/webm"},
		{"mkv", "video/x-matroska"},
		{"mp3", "audio/mpeg"},
		{"pdf", "application/pdf"},
		{"unknown", "application/octet-stream"},
		{"", "application/octet-stream"},
	}

	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			got := guessMimeType(tt.ext)
			if got != tt.want {
				t.Errorf("guessMimeType(%q) = %q, want %q", tt.ext, got, tt.want)
			}
		})
	}
}

func TestJobView(t *testing.T) {
	now := time.Now().UTC()
	j := &Job{
		ID:        "dl_test123",
		URL:       "https://example.com/video.mp4",
		Status:    StatusPending,
		Title:     "Test Video",
		Ext:       "mp4",
		CreatedAt: now,
		UpdatedAt: now,
	}

	v := j.View()
	if v.ID != j.ID {
		t.Errorf("View().ID = %q, want %q", v.ID, j.ID)
	}
	if v.Title != j.Title {
		t.Errorf("View().Title = %q, want %q", v.Title, j.Title)
	}
	if v.Status != StatusPending {
		t.Errorf("View().Status = %q, want %q", v.Status, StatusPending)
	}
}

func TestStorePutGetDelete(t *testing.T) {
	s := NewStore()
	j := &Job{
		ID:        "dl_store_test",
		URL:       "https://example.com/video.mp4",
		Status:    StatusPending,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	s.Put(j)

	got, err := s.Get("dl_store_test")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.ID != j.ID {
		t.Errorf("Get().ID = %q, want %q", got.ID, j.ID)
	}

	// Not found
	_, err = s.Get("nonexistent")
	if err == nil {
		t.Error("Get should return error for nonexistent job")
	}

	// Delete
	s.Delete("dl_store_test")
	_, err = s.Get("dl_store_test")
	if err == nil {
		t.Error("Get should return error after Delete")
	}
}

func TestStoreList(t *testing.T) {
	s := NewStore()
	now := time.Now().UTC()

	j1 := &Job{ID: "dl_1", URL: "https://a.com", Status: StatusPending, CreatedAt: now, UpdatedAt: now}
	j2 := &Job{ID: "dl_2", URL: "https://b.com", Status: StatusCompleted, CreatedAt: now, UpdatedAt: now}
	s.Put(j1)
	s.Put(j2)

	list := s.List()
	if len(list) != 2 {
		t.Fatalf("List() returned %d items, want 2", len(list))
	}
}

func TestStoreUpdate(t *testing.T) {
	s := NewStore()
	now := time.Now().UTC()
	j := &Job{ID: "dl_update", URL: "https://example.com", Status: StatusPending, CreatedAt: now, UpdatedAt: now}
	s.Put(j)

	updated, err := s.Update("dl_update", func(j *Job) error {
		j.Status = StatusDownloading
		j.Progress = 42.5
		return nil
	})
	if err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	if updated.Status != StatusDownloading {
		t.Errorf("Update().Status = %q, want %q", updated.Status, StatusDownloading)
	}
	if updated.Progress != 42.5 {
		t.Errorf("Update().Progress = %f, want 42.5", updated.Progress)
	}
}

func TestStoreDeleteOlderThan(t *testing.T) {
	s := NewStore()
	now := time.Now().UTC()
	old := now.Add(-2 * time.Hour)

	// Terminal job older than 1 hour — should be deleted
	s.Put(&Job{ID: "old_completed", Status: StatusCompleted, CreatedAt: old, UpdatedAt: old})
	// Terminal job recent — should be kept
	s.Put(&Job{ID: "recent_completed", Status: StatusCompleted, CreatedAt: now, UpdatedAt: now})
	// Active job older than 1 hour — should be kept (not terminal)
	s.Put(&Job{ID: "old_downloading", Status: StatusDownloading, CreatedAt: old, UpdatedAt: old})

	n := s.DeleteOlderThan(time.Hour)
	if n != 1 {
		t.Errorf("DeleteOlderThan returned %d, want 1", n)
	}

	// Verify
	if _, err := s.Get("old_completed"); err == nil {
		t.Error("old_completed should have been deleted")
	}
	if _, err := s.Get("recent_completed"); err != nil {
		t.Error("recent_completed should still exist")
	}
	if _, err := s.Get("old_downloading"); err != nil {
		t.Error("old_downloading should still exist (not terminal)")
	}
}

func TestNewID(t *testing.T) {
	id1 := newID()
	id2 := newID()

	if id1 == id2 {
		t.Error("newID should produce unique IDs")
	}
	if len(id1) < 10 {
		t.Errorf("newID() = %q, too short", id1)
	}
	if id1[:3] != "dl_" {
		t.Errorf("newID() = %q, should start with 'dl_'", id1)
	}
}

func TestParseProgressLine(t *testing.T) {
	tests := []struct {
		line           string
		wantPct        float64
		wantSpeed      string
		wantETA        string
		wantDownloaded int64
		wantTotal      int64
	}{
		{
			"PROGRESS:  50.0%|  1.23MiB/s|00:30|1048576|2097152|2097152",
			50.0, "1.23MiB/s", "00:30", 1048576, 2097152,
		},
		{
			"PROGRESS: 100.0%|    0.00B/s|00:00|2097152|2097152|2097152",
			100.0, "0.00B/s", "00:00", 2097152, 2097152,
		},
		// total_bytes is NA, fall back to total_bytes_estimate
		{
			"PROGRESS:  25.0%|  500.00KiB/s|01:00|524288|NA|2097152",
			25.0, "500.00KiB/s", "01:00", 524288, 2097152,
		},
		// Both total fields are NA — total will be 0
		{
			"PROGRESS:  10.0%|  100.00KiB/s|02:00|102400|NA|NA",
			10.0, "100.00KiB/s", "02:00", 102400, 0,
		},
		{
			"not a progress line",
			0, "", "", 0, 0,
		},
	}

	for _, tt := range tests {
		pct, speed, eta, dl, total := parseProgressLine(tt.line)
		if pct != tt.wantPct || speed != tt.wantSpeed || eta != tt.wantETA ||
			dl != tt.wantDownloaded || total != tt.wantTotal {
			t.Errorf("parseProgressLine(%q) = (%f, %q, %q, %d, %d), want (%f, %q, %q, %d, %d)",
				tt.line, pct, speed, eta, dl, total,
				tt.wantPct, tt.wantSpeed, tt.wantETA, tt.wantDownloaded, tt.wantTotal)
		}
	}
}

// ---- Pause/Resume + dedupe + TTL (added with breakpoint-resume feature) ----

func mkTestJob(id, url string, st Status, age time.Duration) *Job {
	now := time.Now().UTC().Add(-age)
	return &Job{
		ID: id, URL: url, Status: st,
		CreatedAt: now, UpdatedAt: now,
		cancelCh: make(chan struct{}),
	}
}

func TestFindExisting_Dedupe(t *testing.T) {
	s := NewStore()
	s.Put(mkTestJob("j1", "https://x/active", StatusDownloading, time.Minute))
	s.Put(mkTestJob("j2", "https://x/fresh-done", StatusCompleted, 30*time.Minute))
	s.Put(mkTestJob("j3", "https://x/stale-done", StatusCompleted, 2*time.Hour))
	s.Put(mkTestJob("j4", "https://x/failed", StatusFailed, time.Minute))
	s.Put(mkTestJob("j5", "https://x/paused", StatusPaused, time.Minute))

	svc := &Service{Store: s, CacheTTL: time.Hour}

	if j, tag := svc.FindExisting("https://x/active"); j == nil || tag != "active" || j.ID != "j1" {
		t.Errorf("active dedupe: got (%v, %q)", j, tag)
	}
	if j, tag := svc.FindExisting("https://x/paused"); j == nil || tag != "active" {
		t.Errorf("paused dedupe: got (%v, %q)", j, tag)
	}
	if j, tag := svc.FindExisting("https://x/fresh-done"); j == nil || tag != "completed" || j.ID != "j2" {
		t.Errorf("completed dedupe: got (%v, %q)", j, tag)
	}
	if j, _ := svc.FindExisting("https://x/stale-done"); j != nil {
		t.Errorf("stale completed job should not dedupe, got %v", j)
	}
	if j, _ := svc.FindExisting("https://x/failed"); j != nil {
		t.Errorf("failed job should not dedupe, got %v", j)
	}
	if j, _ := svc.FindExisting("https://x/unknown"); j != nil {
		t.Errorf("unknown URL should not dedupe, got %v", j)
	}
}

func TestServicePauseResumeGuards(t *testing.T) {
	s := NewStore()
	svc := &Service{Store: s}

	dl := mkTestJob("j1", "https://x/1", StatusDownloading, 0)
	s.Put(dl)

	j, err := svc.Pause("j1")
	if err != nil || j.Status != StatusPaused {
		t.Fatalf("pause downloading: status=%s err=%v", j.Status, err)
	}

	// Pausing an already-paused job is rejected (it is no longer active).
	if _, err := svc.Pause("j1"); err == nil {
		t.Error("pause on paused job should fail")
	}

	// Resume flips it back to pending and (with no yt-dlp binary) the
	// background goroutine will fail the job afterwards — the synchronous
	// transition is what we assert.
	jr, err := svc.Resume("j1")
	if err != nil {
		t.Fatalf("resume: %v", err)
	}
	if jr.Status == StatusPaused {
		t.Error("resume left job paused")
	}

	done := mkTestJob("j2", "https://x/2", StatusCompleted, 0)
	s.Put(done)
	if _, err := svc.Pause("j2"); err == nil {
		t.Error("pause on completed job should fail")
	}
	if _, err := svc.Resume("j2"); err == nil {
		t.Error("resume on completed job should fail")
	}
}

func TestStoreDeleteExpired(t *testing.T) {
	s := NewStore()
	s.Put(mkTestJob("done-old", "u1", StatusCompleted, 2*time.Hour))
	s.Put(mkTestJob("done-new", "u2", StatusCompleted, 30*time.Minute))
	s.Put(mkTestJob("fail-old", "u3", StatusFailed, 2*time.Hour))
	s.Put(mkTestJob("fail-new", "u4", StatusFailed, 10*time.Minute))
	s.Put(mkTestJob("cxl-old", "u5", StatusCancelled, 2*time.Hour))
	s.Put(mkTestJob("paused", "u6", StatusPaused, 2*time.Hour))
	s.Put(mkTestJob("downloading", "u7", StatusDownloading, 2*time.Hour))

	n := s.DeleteExpired(time.Hour, 30*time.Minute)
	if n != 3 {
		t.Fatalf("DeleteExpired removed %d, want 3 (done-old, fail-old, cxl-old)", n)
	}
	for _, id := range []string{"done-new", "fail-new", "paused", "downloading"} {
		if _, err := s.Get(id); err != nil {
			t.Errorf("%s was removed but should have been kept", id)
		}
	}
}
