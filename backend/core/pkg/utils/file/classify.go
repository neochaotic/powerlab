package file

import (
	"path/filepath"
	"strings"
)

// Listing type values returned in the directory listing's `type` field
// (#38). The UI picks a previewer from these instead of re-sniffing the
// extension on the client.
const (
	TypeVideo   = "video"
	TypeAudio   = "audio"
	TypeImage   = "image"
	TypeText    = "text"
	TypePDF     = "pdf"
	TypeArchive = "archive"
	TypeBlob    = "blob"
)

var typeByExtension = map[string]string{
	// video
	"mp4": TypeVideo, "m4v": TypeVideo, "webm": TypeVideo, "mov": TypeVideo,
	"mkv": TypeVideo, "avi": TypeVideo, "wmv": TypeVideo, "flv": TypeVideo,
	"mpg": TypeVideo, "mpeg": TypeVideo, "3gp": TypeVideo,
	// audio
	"mp3": TypeAudio, "flac": TypeAudio, "wav": TypeAudio, "ogg": TypeAudio,
	"oga": TypeAudio, "opus": TypeAudio, "m4a": TypeAudio, "aac": TypeAudio,
	"wma": TypeAudio, "aiff": TypeAudio,
	// image
	"jpg": TypeImage, "jpeg": TypeImage, "png": TypeImage, "gif": TypeImage,
	"webp": TypeImage, "svg": TypeImage, "bmp": TypeImage, "ico": TypeImage,
	"avif": TypeImage, "tif": TypeImage, "tiff": TypeImage, "heic": TypeImage,
	// pdf
	"pdf": TypePDF,
	// archive
	"zip": TypeArchive, "tar": TypeArchive, "gz": TypeArchive, "tgz": TypeArchive,
	"bz2": TypeArchive, "xz": TypeArchive, "zst": TypeArchive, "7z": TypeArchive,
	"rar": TypeArchive, "iso": TypeArchive,
	// text
	"txt": TypeText, "md": TypeText, "markdown": TypeText, "yaml": TypeText,
	"yml": TypeText, "json": TypeText, "toml": TypeText, "ini": TypeText,
	"conf": TypeText, "cfg": TypeText, "env": TypeText, "log": TypeText,
	"csv": TypeText, "tsv": TypeText, "xml": TypeText, "html": TypeText,
	"htm": TypeText, "css": TypeText, "js": TypeText, "mjs": TypeText,
	"ts": TypeText, "jsx": TypeText, "tsx": TypeText, "svelte": TypeText, "vue": TypeText,
	"sh": TypeText, "bash": TypeText, "zsh": TypeText, "py": TypeText,
	"go": TypeText, "rs": TypeText, "rb": TypeText, "php": TypeText,
	"java": TypeText, "kt": TypeText, "c": TypeText, "h": TypeText,
	"cpp": TypeText, "hpp": TypeText, "sql": TypeText, "srt": TypeText,
	"vtt": TypeText, "gitignore": TypeText, "dockerignore": TypeText,
	"dockerfile": TypeText,
}

// Extensionless names that are conventionally text.
var textBaseNames = map[string]bool{
	"dockerfile": true, "makefile": true, "readme": true, "license": true,
	"containerfile": true, "vagrantfile": true,
}

// Extension returns the lowercase extension of name without the dot,
// or "" when there is none. A dotfile with no further dot (".env")
// yields the part after the dot, matching the UI's convention.
func Extension(name string) string {
	ext := filepath.Ext(name)
	if ext == "" || ext == "." {
		return ""
	}
	return strings.ToLower(ext[1:])
}

// Classify maps a file name to one of the listing type values, from
// its extension alone (no disk I/O, so it is cheap for big listings).
// Unknown extensions are TypeBlob.
func Classify(name string) string {
	if t, ok := typeByExtension[Extension(name)]; ok {
		return t
	}
	if textBaseNames[strings.ToLower(filepath.Base(name))] {
		return TypeText
	}
	return TypeBlob
}
