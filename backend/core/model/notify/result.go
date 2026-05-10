package notify

// Notify struct for Notify
type NotifyModel struct {
	Data  interface{} `json:"data"`
	State string      `json:"state"`
}
