package model

type SharesDBModel struct {
	ID        uint   `gorm:"column:id;primary_key" json:"id"`
	Anonymous bool   `json:"anonymous"`
	Path      string `json:"path"`
	Name      string `json:"name"`
	Updated   int64  `gorm:"autoUpdateTime"`
	Created   int64  `gorm:"autoCreateTime"`
}

func (p *SharesDBModel) TableName() string {
	return "o_shares"
}
