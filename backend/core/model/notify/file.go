package notify

type File struct {
	Finished       bool   `json:"finished"`
	ProcessedSize  int64  `json:"processed_size"`
	ProcessingPath string `json:"processing_path"`
	Status         string `json:"status"`
	TotalSize      int64  `json:"total_size"`
	Id             string `json:"id"`
	To             string `json:"to"`
	Type           string `json:"type"`
}
