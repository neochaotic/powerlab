package notify

type StorageMessage struct {
	Type   string `json:"type"`   //sata,usb
	Action string `json:"action"` //remove add
	Path   string `json:"path"`
	Volume string `json:"volume"`
	Size   uint64 `json:"size"`
}
