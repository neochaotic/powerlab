package model

import "time"

//Soon to be removed
type UserDBModel struct {
	Id          int       `gorm:"column:id;primary_key" json:"id"`
	Username    string    `json:"username"`
	Password    string    `json:"password,omitempty"`
	Role        string    `json:"role"`
	Email       string    `json:"email"`
	Nickname    string    `json:"nickname"`
	Avatar      string    `json:"avatar"`
	Description string    `json:"description"`
	Source      string    `json:"source"` // "db" or "os"
	UID         string    `json:"uid"`    // OS UID
	CreatedAt   time.Time `gorm:"<-:create;autoCreateTime" json:"created_at,omitempty"`
	UpdatedAt   time.Time `gorm:"<-:create;<-:update;autoUpdateTime" json:"updated_at,omitempty"`
}

func (p *UserDBModel) TableName() string {
	return "o_users"
}
