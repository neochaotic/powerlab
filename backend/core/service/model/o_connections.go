package model

type ConnectionsDBModel struct {
	ID          uint   `gorm:"column:id;primary_key" json:"id"`
	Updated     int64  `gorm:"autoUpdateTime"`
	Created     int64  `gorm:"autoCreateTime"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	Host        string `json:"host"`
	Port        string `json:"port"`
	Status      string `json:"status"`
	Directories string `json:"directories"` // string array
	MountPoint  string `json:"mount_point"` //parent directory of mount point
}

func (p *ConnectionsDBModel) TableName() string {
	return "o_connections"
}
