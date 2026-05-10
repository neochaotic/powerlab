// Package external is the cross-service SDK: each backend service
// imports this package to talk to its peers (gateway, message-bus,
// user-service, app-management) without having to duplicate URL
// resolution, retry, or auth boilerplate. Address resolution always
// goes through the runtime-path *.url files dropped by the gateway
// at startup.
package external

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/neochaotic/powerlab/backend/common/model"
	http2 "github.com/neochaotic/powerlab/backend/common/utils/http"
)

const (
	ManagementURLFilename = "management.url"
	StaticURLFilename     = "static.url"
	APIGatewayRoutes      = "/v1/gateway/routes"
	APIGatewayPort        = "/v1/gateway/port"
)

// ManagementService is the gateway's management API as seen by
// other backend services. Used at startup to register routes
// (path → upstream) and read/change the listen port.
type ManagementService interface {
	// CreateRoute registers a path → target route on the gateway.
	// Idempotent on the gateway side — backend services call this
	// on every boot.
	CreateRoute(route *model.Route) error

	// ChangePort tells the gateway to rebind on a new port. Used by
	// the admin UI's port-change flow.
	ChangePort(request *model.ChangePortRequest) error

	// GetPort returns the gateway's current listen port. Note the
	// (error, string) return order is legacy — kept for
	// backwards-compatibility with existing callers.
	GetPort() (error, string)
}

type managementService struct {
	address string
}

func (m *managementService) CreateRoute(route *model.Route) error {
	url := strings.TrimSuffix(m.address, "/") + "/" + strings.TrimPrefix(APIGatewayRoutes, "/")
	body, err := json.Marshal(route)
	if err != nil {
		return err
	}

	response, err := http2.Post(url, body, 30*time.Second)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusCreated {
		return errors.New("failed to create route (status code: " + fmt.Sprint(response.StatusCode) + ")")
	}

	return nil
}

func (m *managementService) ChangePort(request *model.ChangePortRequest) error {
	url := strings.TrimSuffix(m.address, "/") + "/" + strings.TrimPrefix(APIGatewayPort, "/")
	body, err := json.Marshal(request)
	if err != nil {
		return err
	}

	response, err := http2.Put(url, body, 30*time.Second)
	if err != nil {
		return err
	}

	if response.StatusCode != http.StatusOK {
		return errors.New("failed to change port (status code: " + fmt.Sprint(response.StatusCode) + ")")
	}

	return nil
}

func (m *managementService) GetPort() (error, string) {
	url := strings.TrimSuffix(m.address, "/") + "/" + strings.TrimPrefix(APIGatewayPort, "/")

	response, err := http2.Get(url, 30*time.Second)
	if err != nil {
		return err, ""
	}

	if response.StatusCode != http.StatusOK {
		return errors.New("failed to change port (status code: " + fmt.Sprint(response.StatusCode) + ")"), ""
	}
	str, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return err, ""
	}
	return nil, string(str)
}

// NewManagementService resolves the gateway management URL from
// management.url under RuntimePath, retries up to ~10s for the file
// to appear (gateway may still be writing it during co-startup),
// pings /ping for readiness, and returns a ready-to-use
// ManagementService.
func NewManagementService(RuntimePath string) (ManagementService, error) {
	managementAddressFile := filepath.Join(RuntimePath, ManagementURLFilename)

	retry := 10

	for retry > 0 {
		if _, err := os.Stat(managementAddressFile); err == nil {
			break
		}

		fmt.Printf("gateway management address file `%s` not found, retrying in 1 second...(%d)\n", managementAddressFile, retry)

		time.Sleep(1 * time.Second)

		retry--
	}

	address, err := getAddress(managementAddressFile)
	if err != nil {
		return nil, err
	}

	if err := ping(address, 30*time.Second); err != nil {
		return nil, err
	}

	return &managementService{
		address: address,
	}, nil
}
