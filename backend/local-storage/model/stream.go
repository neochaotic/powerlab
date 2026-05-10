package model

import (
	"io"
)

// FileStream wraps an Obj with a live ReadCloser body — used by
// upload + put paths. Old carries the previous version when a put
// is overwriting an existing file (drivers may want it for atomic
// rename + delete).
type FileStream struct {
	Obj
	io.ReadCloser
	Mimetype     string
	WebPutAsTask bool
	Old          Obj
}

// GetMimetype returns the upload's content-type header, or empty
// string if none was provided.
func (f *FileStream) GetMimetype() string {
	return f.Mimetype
}

// NeedStore reports whether this upload should be persisted via the
// async task queue rather than synchronously inline.
func (f *FileStream) NeedStore() bool {
	return f.WebPutAsTask
}

// GetReadCloser returns the underlying body. Caller must Close.
func (f *FileStream) GetReadCloser() io.ReadCloser {
	return f.ReadCloser
}

// SetReadCloser swaps the body — used by middlewares that buffer or
// transform the upload (e.g. progress bar wrappers).
func (f *FileStream) SetReadCloser(rc io.ReadCloser) {
	f.ReadCloser = rc
}

// GetOld returns the previous Obj when this stream is replacing an
// existing file, nil otherwise.
func (f *FileStream) GetOld() Obj {
	return f.Old
}
