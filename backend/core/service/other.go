package service

import (
	"time"

	"github.com/neochaotic/powerlab/backend/common/utils/logger"
	"github.com/go-resty/resty/v2"
	"go.uber.org/zap"
)

// OtherService is a small grab-bag of utility methods that don't
// belong to a more specific service. Currently exposes a generic
// HTTP-fetch proxy used by the UI's search widget.
type OtherService interface {
	AgentSearch(url string) ([]byte, error)
}

type otherService struct{}

// AgentSearch fetches an arbitrary URL server-side on behalf of the
// UI. Avoids CORS friction for endpoints that don't set permissive
// CORS headers themselves. 3-second timeout — caller's responsibility
// to handle transient failures.
//
// NOTE: this is a SSRF-shaped helper — it'll fetch any URL the
// caller passes. Behind /v1/recommend so JWT auth gates access.
// If exposing more broadly, add an allowlist.
func (s *otherService) AgentSearch(url string) ([]byte, error) {
	client := resty.New()
	client.SetTimeout(3 * time.Second)
	resp, err := client.R().Get(url)
	if err != nil {
		logger.Error("AgentSearch fetch error", zap.Error(err), zap.String("url", url))
		return nil, err
	}
	return resp.Body(), nil
}

// NewOtherService constructs a fresh OtherService.
func NewOtherService() OtherService {
	return &otherService{}
}
