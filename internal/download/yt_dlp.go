package download

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// YtDlpConfig holds the runtime configuration for yt-dlp invocations.
type YtDlpConfig struct {
	BinPath     string // path to yt-dlp executable
	Proxy       string // socks5://127.0.0.1:10808 or empty
	CookiePath  string // path to Netscape cookie file or empty
	TmpDir      string // directory for downloaded temp files
}

// progressLine matches "PROGRESS:  0.1%|  39.33KiB/s|01:25|3072|3433755"
var progressRe = regexp.MustCompile(`^PROGRESS:\s*([\d.]+)%\|([^|]*)\|([^|]*)\|(\d+)\|([^|]*)\|([^|]*)`)

// metaInfo is the subset of yt-dlp -j output we care about.
type metaInfo struct {
	ID            string  `json:"id"`
	Title         string  `json:"title"`
	Ext           string  `json:"ext"`
	Duration      float64 `json:"duration"`
	Filesize      int64   `json:"filesize"`
	FilesizeApprox int64  `json:"filesize_approx"`
	Resolution    string  `json:"resolution"`
	Thumbnail     string  `json:"thumbnail"`
	Uploader      string  `json:"uploader"`
	WebpageURL    string  `json:"webpage_url"`
	Extractor     string  `json:"extractor"`
}

// ResolveMetadata runs yt-dlp --simulate -j to extract metadata without downloading.
func ResolveMetadata(ctx context.Context, cfg YtDlpConfig, rawURL string) (metaInfo, error) {
	args := buildBaseArgs(cfg, rawURL)
	args = append(args, "--simulate", "-j")

	cmd := exec.CommandContext(ctx, cfg.BinPath, args...)
	output, err := cmd.Output()
	if err != nil {
		// Try to extract stderr message
		var ee *exec.ExitError
		if exitErr, ok := err.(*exec.ExitError); ok {
			ee = exitErr
		}
		msg := err.Error()
		if ee != nil && len(ee.Stderr) > 0 {
			msg = strings.TrimSpace(string(ee.Stderr))
			// Extract last ERROR: line
			for _, line := range strings.Split(msg, "\n") {
				if strings.Contains(line, "ERROR:") {
					msg = strings.TrimSpace(line)
					break
				}
			}
		}
		return metaInfo{}, fmt.Errorf("yt-dlp: %s", msg)
	}

	var meta metaInfo
	if err := json.Unmarshal(output, &meta); err != nil {
		return metaInfo{}, fmt.Errorf("parse yt-dlp metadata: %w", err)
	}
	if meta.Title == "" {
		// For generic/direct links, title may be empty — use filename
		meta.Title = filepath.Base(rawURL)
	}
	return meta, nil
}

// DownloadResult contains the path of the downloaded file and metadata.
type DownloadResult struct {
	FilePath string
	Meta     metaInfo
}

// Download runs yt-dlp to download the file to tmpDir, calling onProgress for
// each progress update. The context cancellation (or job cancel) will kill the
// subprocess.
func Download(ctx context.Context, cfg YtDlpConfig, rawURL string, onProgress func(pct float64, speed, eta string, downloaded, total int64)) (*DownloadResult, error) {
	// First, resolve metadata so we know the title/ext for the output template.
	meta, err := ResolveMetadata(ctx, cfg, rawURL)
	if err != nil {
		return nil, err
	}

	// Build output template: tmpDir/title.ext
	// Sanitize title for filesystem safety.
	safeTitle := sanitizeFilename(meta.Title)
	if safeTitle == "" {
		safeTitle = "download"
	}
	ext := meta.Ext
	if ext == "" || ext == "unknown_video" {
		// For direct links, yt-dlp often returns "unknown_video" as ext.
		// Try to infer from URL; if that also fails, default to mp4.
		ext = guessExt(rawURL)
	}

	// Build output template. When yt-dlp returns "unknown_video", we cannot
	// use %(ext)s because yt-dlp will literally produce a file named
	// "title.unknown_video". Instead, use the inferred extension directly.
	var outTemplate string
	if meta.Ext == "" || meta.Ext == "unknown_video" {
		outTemplate = filepath.Join(cfg.TmpDir, safeTitle+"."+ext)
	} else {
		outTemplate = filepath.Join(cfg.TmpDir, safeTitle+".%(ext)s")
	}

	args := buildBaseArgs(cfg, rawURL)
	args = append(args,
		"--newline",
		"--progress-template", "PROGRESS:%(progress._percent_str)s|%(progress._speed_str)s|%(progress._eta_str)s|%(progress.downloaded_bytes)s|%(progress._total_bytes_estimate)s|%(progress._total_bytes)s",
		"--no-playlist",
		"--no-part",
		"-o", outTemplate,
	)

	cmd := exec.CommandContext(ctx, cfg.BinPath, args...)

	// Capture stdout for progress, stderr for errors.
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("pipe stdout: %w", err)
	}
	cmd.Stderr = cmd.Stdout // merge stderr into stdout so we see ERROR lines too

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start yt-dlp: %w", err)
	}

	// Read progress lines.
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	var lastErrLine string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "PROGRESS:") {
			pct, speed, eta, downloaded, total := parseProgressLine(line)
			if onProgress != nil {
				onProgress(pct, speed, eta, downloaded, total)
			}
			continue
		}
		if strings.Contains(line, "ERROR:") {
			lastErrLine = strings.TrimSpace(line)
		}
		// Also capture "[Merger] Merging formats into" to know the final filename
		if strings.Contains(line, "Merging formats into") || strings.Contains(line, "Destination:") {
			// Extract the file path from the line
			if idx := strings.Index(line, "\""); idx >= 0 {
				_ = line[idx+1:] // just acknowledge; final path resolved below
			}
		}
	}

	if err := cmd.Wait(); err != nil {
		msg := err.Error()
		if lastErrLine != "" {
			msg = lastErrLine
		}
		return nil, fmt.Errorf("yt-dlp: %s", msg)
	}

	// Find the downloaded file — yt-dlp may have changed the extension during merge.
	filePath := findDownloadedFile(cfg.TmpDir, safeTitle, ext)
	if filePath == "" {
		// Fallback 1: try glob with just the title prefix
		pattern := filepath.Join(cfg.TmpDir, safeTitle+".*")
		matches, _ := filepath.Glob(pattern)
		filePath = pickLargestFile(matches)
	}
	if filePath == "" {
		// Fallback 2: scan the entire tmpDir for the largest media file.
		// This handles cases where yt-dlp ignores the -o template and uses
		// the URL filename instead (common for direct links).
		filePath = scanDirForLargestMedia(cfg.TmpDir)
	}
	if filePath == "" {
		return nil, fmt.Errorf("yt-dlp completed but output file not found in %s", cfg.TmpDir)
	}

	return &DownloadResult{
		FilePath: filePath,
		Meta:     meta,
	}, nil
}

// buildBaseArgs constructs the common yt-dlp arguments shared by ResolveMetadata and Download.
func buildBaseArgs(cfg YtDlpConfig, rawURL string) []string {
	args := []string{"--no-warnings"}

	// Proxy: only add if the domain needs it. For domestic sites, explicitly
	// pass --proxy "" to override any HTTP_PROXY/HTTPS_PROXY env vars that the
	// parent process (Go server) may have set — otherwise yt-dlp inherits them
	// and routes domestic traffic through the proxy, causing 412/reset errors.
	if ShouldUseProxy(rawURL, cfg.Proxy) {
		args = append(args, "--proxy", cfg.Proxy)
	} else {
		args = append(args, "--proxy", "")
	}

	// Cookies: add for domains that require them.
	if NeedsCookies(rawURL) && cfg.CookiePath != "" {
		args = append(args, "--cookies", cfg.CookiePath)
	}

	args = append(args, rawURL)
	return args
}

// parseProgressLine extracts numeric fields from a PROGRESS: line.
// Format: PROGRESS:pct|speed|eta|downloaded|total_estimate|total_bytes
// total_bytes may be "NA" (unknown); total_estimate is used as fallback.
func parseProgressLine(line string) (pct float64, speed, eta string, downloaded, total int64) {
	m := progressRe.FindStringSubmatch(line)
	if m == nil {
		return 0, "", "", 0, 0
	}
	pct, _ = strconv.ParseFloat(strings.TrimSpace(m[1]), 64)
	speed = strings.TrimSpace(m[2])
	eta = strings.TrimSpace(m[3])
	downloaded, _ = strconv.ParseInt(m[4], 10, 64)
	// Prefer total_bytes (m[6]); fall back to total_bytes_estimate (m[5]).
	total, _ = strconv.ParseInt(strings.TrimSpace(m[6]), 10, 64)
	if total == 0 {
		total, _ = strconv.ParseInt(strings.TrimSpace(m[5]), 10, 64)
	}
	return
}

// sanitizeFilename removes characters that are invalid in Windows/Linux filenames.
func sanitizeFilename(name string) string {
	name = strings.TrimSpace(name)
	// Replace invalid characters with underscore
	replacer := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		":", "_",
		"*", "_",
		"?", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
	)
	name = replacer.Replace(name)
	// Collapse multiple underscores
	for strings.Contains(name, "__") {
		name = strings.ReplaceAll(name, "__", "_")
	}
	// Limit length to 150 chars to avoid filesystem path limits.
	if len(name) > 150 {
		name = name[:150]
	}
	return name
}

// guessExt tries to infer a file extension from the URL path.
// It checks the URL path for known extensions; if none found, defaults to "mp4".
func guessExt(rawURL string) string {
	s := rawURL
	// Strip query string
	if idx := strings.Index(s, "?"); idx >= 0 {
		s = s[:idx]
	}
	// Remove trailing slash
	s = strings.TrimRight(s, "/")
	ext := filepath.Ext(s)
	ext = strings.TrimPrefix(ext, ".")
	ext = strings.ToLower(ext)

	// Normalize known aliases
	switch ext {
	case "":
		// No extension in URL — default to mp4 for video, but this may be
		// wrong for non-video direct links. The caller should handle this.
		ext = "mp4"
	case "htm", "html":
		ext = "mp4" // HTML page, not a direct file — default to mp4
	}
	return ext
}

// findDownloadedFile searches for the downloaded file by title and extension.
// yt-dlp may produce a different extension than expected (e.g. webm instead of mp4).
func findDownloadedFile(dir, title, ext string) string {
	// Try exact match first
	candidate := filepath.Join(dir, title+"."+ext)
	if isRegularFile(candidate) {
		return candidate
	}
	// Try common media extensions
	mediaExts := []string{
		"mp4", "webm", "mkv", "mp3", "m4a", "opus",
		"epub", "pdf", "zip", "webp", "jpg", "png",
		"unknown_video", // yt-dlp's fallback for direct links
	}
	for _, e := range mediaExts {
		c := filepath.Join(dir, title+"."+e)
		if isRegularFile(c) {
			return c
		}
	}
	return ""
}

// pickLargestFile returns the path of the largest file in the matches slice.
// Ignores .part, .temp, .ytdl files.
func pickLargestFile(matches []string) string {
	var bestPath string
	var bestSize int64
	for _, m := range matches {
		// Skip temporary files.
		lower := strings.ToLower(m)
		if strings.HasSuffix(lower, ".part") || strings.HasSuffix(lower, ".temp") ||
			strings.HasSuffix(lower, ".ytdl") || strings.HasSuffix(lower, ".tmp") {
			continue
		}
		fi, err := statFile(m)
		if err != nil {
			continue
		}
		if fi.Size() > bestSize {
			bestPath = m
			bestSize = fi.Size()
		}
	}
	return bestPath
}

// scanDirForLargestMedia scans the entire directory for the largest file
// modified within the last 5 minutes. This is a last-resort fallback for
// cases where yt-dlp ignores the -o template (common with direct links).
func scanDirForLargestMedia(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	type candidate struct {
		path    string
		size    int64
		modTime time.Time
	}
	var candidates []candidate
	cutoff := time.Now().Add(-5 * time.Minute)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := strings.ToLower(entry.Name())
		// Skip temp/metadata files.
		if strings.HasSuffix(name, ".part") || strings.HasSuffix(name, ".temp") ||
			strings.HasSuffix(name, ".ytdl") || strings.HasSuffix(name, ".tmp") ||
			strings.HasSuffix(name, ".json") || strings.HasSuffix(name, ".webp") ||
			strings.HasSuffix(name, ".jpg") || strings.HasSuffix(name, ".png") ||
			strings.HasSuffix(name, ".srt") || strings.HasSuffix(name, ".vtt") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			continue
		}
		if info.Size() < 1024 { // skip tiny files (< 1KB)
			continue
		}
		candidates = append(candidates, candidate{
			path:    filepath.Join(dir, entry.Name()),
			size:    info.Size(),
			modTime: info.ModTime(),
		})
	}
	if len(candidates) == 0 {
		return ""
	}
	// Sort by size descending; break ties by most recently modified.
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].size != candidates[j].size {
			return candidates[i].size > candidates[j].size
		}
		return candidates[i].modTime.After(candidates[j].modTime)
	})
	return candidates[0].path
}

// isRegularFile returns true if path exists and is not a directory.
func isRegularFile(path string) bool {
	fi, err := statFile(path)
	return err == nil && !fi.IsDir()
}
