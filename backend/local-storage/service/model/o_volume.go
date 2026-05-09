package model

type Volume struct {
	ID         uint   `gorm:"column:id;primary_key" json:"id"`
	UUID       string `json:"uuid"`
	MountPoint string `json:"mount_point"`
	CreatedAt  int64  `json:"created_at"`
}

func (p *Volume) TableName() string {
	return "o_disk" // legacy table name - :(
}
