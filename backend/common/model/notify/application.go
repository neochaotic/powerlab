package notify

type Application struct {
	Name       string            `json:"name"`
	State      string            `json:"state"`
	Type       string            `json:"type"`
	Icon       string            `json:"icon"`
	Message    string            `json:"message"`
	Finished   bool              `json:"finished"`
	Success    bool              `json:"success"`
	Properties map[string]string `json:"properties"`
}
