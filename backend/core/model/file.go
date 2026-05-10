package model

type FileOperate struct {
	Type          string     `json:"type" binding:"required"`
	Item          []FileItem `json:"item" binding:"required"`
	TotalSize     int64      `json:"total_size"`
	ProcessedSize int64      `json:"processed_size"`
	To            string     `json:"to" binding:"required"`
	Style         string     `json:"style"`
	Finished      bool       `json:"finished"`
}

type FileItem struct {
	From          string `json:"from" binding:"required"`
	Finished      bool   `json:"finished"`
	Size          int64  `json:"size"`
	ProcessedSize int64  `json:"processed_size"`
}

// FileUpdate is the body of PUT /v1/file. The JSON keys must match what
// the frontend sends (`file_path`, `file_content` — see ui/src/lib/api/files.ts
// updateFileContent). Earlier versions used `path` / `content` here, which
// caused the binding to silently zero out both fields and the handler
// returned "File already exists" on every save.
type FileUpdate struct {
	FilePath    string `json:"file_path" binding:"required"`
	FileContent string `json:"file_content" binding:"required"`
}
