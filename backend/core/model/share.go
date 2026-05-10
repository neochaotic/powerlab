package model

type Shares struct {
	ID        uint   `json:"id"`
	Anonymous bool   `json:"anonymous"`
	Path      string `json:"path"`
}
