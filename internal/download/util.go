package download

import "os"

// statFile is a thin wrapper around os.Stat for use in yt_dlp.go.
func statFile(path string) (os.FileInfo, error) {
	return os.Stat(path)
}
