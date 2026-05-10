package model

type DockerStatsModel struct {
	Icon     string      `json:"icon"`
	Title    string      `json:"title"`
	Data     interface{} `json:"data"`
	Previous interface{} `json:"previous"`
}

// reference - https://docs.docker.com/engine/reference/commandline/dockerd/#daemon-configuration-file
type DockerDaemonConfigurationModel struct {
	// e.g. `/var/lib/docker`
	Root string `json:"data-root,omitempty"`
}
