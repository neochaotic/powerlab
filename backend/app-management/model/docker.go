package model

// DockerStatsModel is the stats-widget envelope used by the
// homepage dashboard cards (CPU, memory, network, disk per
// container). Data + Previous let the UI render a delta sparkline.
type DockerStatsModel struct {
	Icon     string      `json:"icon"`
	Title    string      `json:"title"`
	Data     interface{} `json:"data"`
	Previous interface{} `json:"previous"`
}

// DockerDaemonConfigurationModel is the typed view of the daemon's
// /etc/docker/daemon.json — only the data-root field today, since
// that's the only field the storage-relocation flow needs to read.
//
// reference: https://docs.docker.com/engine/reference/commandline/dockerd/#daemon-configuration-file
type DockerDaemonConfigurationModel struct {
	// e.g. `/var/lib/docker`
	Root string `json:"data-root,omitempty"`
}
