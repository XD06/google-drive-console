// Package upload tracks resumable upload jobs and progress for the API layer.
// Client chunks are accepted over HTTP; the service re-packs to Drive 256 KiB
// multiples and completes the final partial block.
package upload