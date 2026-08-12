package api

import "testing"

func TestIsAllowedThumbnailURL(t *testing.T) {
	cases := []struct {
		raw  string
		want bool
	}{
		{"https://lh3.googleusercontent.com/a/thumb=s220", true},
		{"https://ggpht.com/a/b", true},
		{"https://www.googleapis.com/drive/v3/files/x/thumbnail", true},
		{"https://drive.google.com/thumbnail?id=x", true},
		{"http://lh3.googleusercontent.com/a", false}, // not https
		{"https://evil.com", false},
		{"https://evil.com?.googleapis.com", false},
		{"https://googleapis.com.evil.com/x", false},
		{"https://user@evil.com/", false},
		{"https://lh3.googleusercontent.com.evil.com/x", false},
		{"not-a-url", false},
		{"", false},
		{"https://", false},
	}
	for _, tc := range cases {
		got := isAllowedThumbnailURL(tc.raw)
		if got != tc.want {
			t.Errorf("isAllowedThumbnailURL(%q) = %v, want %v", tc.raw, got, tc.want)
		}
	}
}
