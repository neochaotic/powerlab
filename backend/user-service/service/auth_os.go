package service

import (
	"errors"
	"os/exec"
	"os/user"
	"runtime"
)

type OSService struct{}

func (s *OSService) Authenticate(username, password string) (bool, error) {
	// 1. Check if user exists in OS
	_, err := user.Lookup(username)
	if err != nil {
		return false, errors.New("user does not exist in system")
	}

	// 2. Validate password
	if runtime.GOOS == "darwin" {
		return s.authenticateMacOS(username, password)
	}
	if runtime.GOOS == "linux" {
		return s.authenticateLinux(username, password)
	}

	return false, errors.New("unsupported operating system for OS auth")
}

func (s *OSService) authenticateMacOS(username, password string) (bool, error) {
	// Use dscl to verify password
	// dscl . -authonly <user> <password>
	cmd := exec.Command("dscl", ".", "-authonly", username, password)
	err := cmd.Run()
	if err != nil {
		return false, errors.New("invalid credentials")
	}
	return true, nil
}

func (s *OSService) authenticateLinux(username, password string) (bool, error) {
	// On Linux, we should ideally use PAM. 
	// For now, let's use a placeholder or try to exec a helper if available.
	// In a real PowerLab deployment, we would use CGO + PAM.
	return false, errors.New("PAM authentication for Linux not yet implemented in this build")
}

func (s *OSService) GetOSUser(username string) (*user.User, error) {
	return user.Lookup(username)
}
