package samba

import (
	"errors"
	"net"

	"github.com/hirochachacha/go-smb2"
)

func ConnectSambaService(host, port, username, password, directory string) error {
	conn, err := net.Dial("tcp", host+":"+port)
	if err != nil {
		return err
	}
	defer conn.Close()
	d := &smb2.Dialer{
		Initiator: &smb2.NTLMInitiator{
			User:     username,
			Password: password,
		},
	}

	s, err := d.Dial(conn)
	if err != nil {
		return err
	}
	defer s.Logoff()
	names, err := s.ListSharenames()
	if err != nil {
		return err
	}

	for _, name := range names {
		if name == directory {
			return nil
		}
	}
	return errors.New("directory not found")
}

//get share name list
func GetSambaSharesList(host, port, username, password string) ([]string, error) {
	conn, err := net.Dial("tcp", host+":"+port)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	d := &smb2.Dialer{
		Initiator: &smb2.NTLMInitiator{
			User:     username,
			Password: password,
		},
	}

	s, err := d.Dial(conn)
	if err != nil {
		return nil, err
	}
	defer s.Logoff()
	names, err := s.ListSharenames()
	if err != nil {
		return nil, err
	}
	return names, err
}
