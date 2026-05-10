package model

type ServerCategoryList struct {
	Item []Category `json:"item"`
}
type Category struct {
	ID uint `gorm:"column:id;primary_key" json:"id"`
	//CreatedAt time.Time `json:"created_at"`
	//
	//UpdatedAt time.Time `json:"updated_at"`
	Font  string `json:"font"` // @tiger - 如果这个和前端有关，应该不属于后端的出参范围，而是前端去界定
	Name  string `json:"name"`
	Count uint   `json:"count"` // @tiger - count 属于动态信息，应该单独放在一个出参结构中（原因见另外一个关于 静态/动态 出参的注释）
}
