// Package classify detects whether a torrent's payload is playable video or a
// compressed/unplayable release (SPEC §6.5).
package classify

import (
	"path"
	"regexp"
	"strings"
)

var videoExt = map[string]bool{
	".mkv": true, ".mp4": true, ".avi": true, ".m4v": true,
	".ts": true, ".webm": true, ".mov": true,
}

var compressedExt = map[string]bool{
	".rar": true, ".zip": true, ".7z": true, ".tar": true, ".gz": true,
}

// split-archive patterns: .r00-.r99 and .001-.999
var splitRe = regexp.MustCompile(`(?i)\.(r\d{2}|\d{3})$`)

// File is the minimal view classify needs.
type File struct {
	Path string
	Size int64
}

// Result reports playability and a human warning when relevant.
type Result struct {
	Playable bool
	Warning  string
}

// Classify inspects the file list and decides if it is playable.
// Rule: playable if a video extension is the dominant content by bytes;
// compressed/unplayable if the payload is mainly archives or has no video.
func Classify(files []File) Result {
	var videoBytes, compressedBytes, totalBytes int64
	hasVideo := false
	for _, f := range files {
		ext := strings.ToLower(path.Ext(f.Path))
		totalBytes += f.Size
		switch {
		case videoExt[ext]:
			hasVideo = true
			videoBytes += f.Size
		case compressedExt[ext] || splitRe.MatchString(f.Path):
			compressedBytes += f.Size
		}
	}

	if !hasVideo && compressedBytes > 0 {
		return Result{Playable: false, Warning: "This release is a compressed archive (RAR/ZIP/split) and cannot be streamed. Extract it first or pick a non-archived release."}
	}
	if !hasVideo {
		return Result{Playable: false, Warning: "No playable video file found in this torrent."}
	}
	// Video present but archives dominate the payload.
	if totalBytes > 0 && compressedBytes > videoBytes {
		return Result{Playable: false, Warning: "This release is mostly compressed archives; the video content is incomplete or packed."}
	}
	return Result{Playable: true}
}
