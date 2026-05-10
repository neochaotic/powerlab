package model

type Connections struct {
	ID         uint   `json:"id"`
	Username   string `json:"username"`
	Password   string `json:"password,omitempty"`
	Host       string `json:"host"`
	Port       string `json:"port"`
	MountPoint string `json:"mount_point"`
}
