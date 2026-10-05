package file

import "testing"

// #38: the directory listing classifies entries server-side so the UI
// does not re-sniff extensions.
func TestClassify(t *testing.T) {
	cases := map[string]string{
		"movie.mkv":       TypeVideo,
		"clip.MP4":        TypeVideo,
		"song.flac":       TypeAudio,
		"track.mp3":       TypeAudio,
		"photo.JPEG":      TypeImage,
		"logo.svg":        TypeImage,
		"manual.pdf":      TypePDF,
		"backup.tar.gz":   TypeArchive,
		"bundle.zip":      TypeArchive,
		"notes.txt":       TypeText,
		"compose.yaml":    TypeText,
		"main.go":         TypeText,
		"index.ts":        TypeText,
		"Dockerfile":      TypeText,
		"Makefile":        TypeText,
		".env":            TypeText,
		"firmware.bin":    TypeBlob,
		"weird.unknownxx": TypeBlob,
		"noext":           TypeBlob,
		"trailingdot.":    TypeBlob,
	}
	for name, want := range cases {
		if got := Classify(name); got != want {
			t.Errorf("Classify(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestExtension(t *testing.T) {
	cases := map[string]string{
		"movie.MKV":     "mkv",
		"backup.tar.gz": "gz",
		".env":          "env",
		"noext":         "",
		"trailingdot.":  "",
	}
	for name, want := range cases {
		if got := Extension(name); got != want {
			t.Errorf("Extension(%q) = %q, want %q", name, got, want)
		}
	}
}
