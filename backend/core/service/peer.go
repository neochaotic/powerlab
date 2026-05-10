package service

import (
	"github.com/neochaotic/powerlab/backend/core/service/model"
	model2 "github.com/neochaotic/powerlab/backend/core/service/model"
	"gorm.io/gorm"
)

// PeerService manages the local-network device-pairing registry —
// the rows shown on the "Other devices" panel of the home screen.
// Identity is keyed on user-agent (for browsers) or display name
// (for native peers).
type PeerService interface {
	GetPeerByUserAgent(ua string) (m model2.PeerDriveDBModel)
	GetPeerByID(id string) (m model2.PeerDriveDBModel)
	GetPeers() (peers []model2.PeerDriveDBModel)
	CreatePeer(m *model2.PeerDriveDBModel)
	DeletePeer(id string)
	GetPeerByName(name string) (m model2.PeerDriveDBModel)
}

type peerStruct struct {
	db *gorm.DB
}

func (s *peerStruct) GetPeerByName(name string) (m model2.PeerDriveDBModel) {
	s.db.Where("display_name = ?", name).First(&m)
	return
}
func (s *peerStruct) GetPeerByUserAgent(ua string) (m model2.PeerDriveDBModel) {
	s.db.Where("user_agent = ?", ua).First(&m)
	return
}
func (s *peerStruct) GetPeerByID(id string) (m model2.PeerDriveDBModel) {
	s.db.Where("id = ?", id).First(&m)
	return
}
func (s *peerStruct) GetPeers() (peers []model2.PeerDriveDBModel) {
	s.db.Order("updated desc").Find(&peers)
	return
}
func (s *peerStruct) CreatePeer(m *model2.PeerDriveDBModel) {

	s.db.Create(m)
}

func (s *peerStruct) DeletePeer(id string) {
	s.db.Where("id= ?", id).Delete(&model.PeerDriveDBModel{})
}

// NewPeerService returns a PeerService backed by db.
func NewPeerService(db *gorm.DB) PeerService {
	return &peerStruct{db: db}
}
