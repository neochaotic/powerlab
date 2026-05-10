package service

import (
	model2 "github.com/neochaotic/powerlab/backend/core/service/model"
	"gorm.io/gorm"
)

// RelyService is the per-app dependency tracker — used by the
// install/upgrade gates to answer "can app X be installed?". Each
// row records what an app needs (driver, port, system feature)
// keyed on the app's CustomID.
type RelyService interface {
	Create(rely model2.RelyDBModel)
	Delete(id string)
	GetInfo(id string) model2.RelyDBModel
}

type relyService struct {
	db *gorm.DB
}

func (r *relyService) Create(rely model2.RelyDBModel) {

	r.db.Create(&rely)

}

// 获取我的应用列表
func (r *relyService) GetInfo(id string) model2.RelyDBModel {
	var m model2.RelyDBModel
	r.db.Where("custom_id = ?", id).First(&m)

	// @tiger - 作为出参不应该直接返回数据库内的格式（见类似问题的注释）
	return m
}

func (r *relyService) Delete(id string) {
	var c model2.RelyDBModel
	r.db.Where("custom_id = ?", id).Delete(&c)
}

// NewRelyService returns a RelyService backed by db.
func NewRelyService(db *gorm.DB) RelyService {
	return &relyService{db: db}
}
