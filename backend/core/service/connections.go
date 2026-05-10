package service

import (
	"fmt"

	"github.com/neochaotic/powerlab/backend/core/service/model"
	model2 "github.com/neochaotic/powerlab/backend/core/service/model"
	"gorm.io/gorm"
)

type ConnectionsService interface {
	GetConnectionsList() (connections []model2.ConnectionsDBModel)
	GetConnectionByHost(host string) (connections []model2.ConnectionsDBModel)
	GetConnectionByID(id string) (connections model2.ConnectionsDBModel)
	CreateConnection(connection *model2.ConnectionsDBModel)
	DeleteConnection(id string)
	UpdateConnection(connection *model2.ConnectionsDBModel)
	MountSmaba(username, host, directory, port, mountPoint, password string) error
	UnmountSmaba(mountPoint string) error
}

type connectionsStruct struct {
	db *gorm.DB
}

func (s *connectionsStruct) GetConnectionByHost(host string) (connections []model2.ConnectionsDBModel) {
	s.db.Select("username,host,status,id").Where("host = ?", host).Find(&connections)
	return
}

func (s *connectionsStruct) GetConnectionByID(id string) (connections model2.ConnectionsDBModel) {
	s.db.Select("username,password,host,status,id,directories,mount_point,port").Where("id = ?", id).First(&connections)
	return
}

func (s *connectionsStruct) GetConnectionsList() (connections []model2.ConnectionsDBModel) {
	s.db.Select("username,host,port,status,id,mount_point").Find(&connections)
	return
}

func (s *connectionsStruct) CreateConnection(connection *model2.ConnectionsDBModel) {
	s.db.Create(connection)
}

func (s *connectionsStruct) UpdateConnection(connection *model2.ConnectionsDBModel) {
	s.db.Save(connection)
}

func (s *connectionsStruct) DeleteConnection(id string) {
	s.db.Where("id= ?", id).Delete(&model.ConnectionsDBModel{})
}

func (s *connectionsStruct) MountSmaba(username, host, directory, port, mountPoint, password string) error {
	// Stubbed for macOS compatibility
	return fmt.Errorf("SMB mounting is not supported on macOS local testing")
}

func (s *connectionsStruct) UnmountSmaba(mountPoint string) error {
	// Stubbed for macOS compatibility
	return fmt.Errorf("SMB unmounting is not supported on macOS local testing")
}

func NewConnectionsService(db *gorm.DB) ConnectionsService {
	return &connectionsStruct{db: db}
}
