package external

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/neochaotic/powerlab/backend/common/model"
	http2 "github.com/neochaotic/powerlab/backend/common/utils/http"
	"github.com/tidwall/gjson"
)

const (
	AppManageURLFilename = "app-management.url"
	APIComposeInfo       = "/v2/app_management/compose"
	APIComposeStatus     = "/v2/app_management/compose"
)

// AppManageService is the app-management API surface as seen by
// peer services. Used by core + gateway to look up app metadata
// and toggle running state without re-implementing the HTTP
// client.
type AppManageService interface {
	// GetAppInfo returns the joined compose-app + store-info view
	// for the given store id, or a zero-value struct + error.
	GetAppInfo(storeId string) (model.ComposeAppWithStoreInfo, error)

	// PutAppStatus sets an installed app's lifecycle status
	// (e.g. "started", "stopped"). Returns true on 200; the bool
	// is redundant with err == nil and kept for backwards-compat.
	PutAppStatus(storeId string, status string) (bool, error)
}

type appManageService struct {
	address string
}

func (m *appManageService) GetAppInfo(storeId string) (model.ComposeAppWithStoreInfo, error) {
	url := strings.TrimSuffix(m.address, "/") + APIComposeInfo + "/" + storeId
	model := model.ComposeAppWithStoreInfo{}
	response, err := http2.Get(url, 5*time.Second)
	if err != nil {
		return model, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return model, errors.New("failed to create route (status code: " + fmt.Sprint(response.StatusCode) + ")")
	}
	str, err := io.ReadAll(response.Body)
	if err != nil {
		return model, err
	}
	defer response.Body.Close()
	gstr := gjson.Get(string(str), "data")
	err = json.Unmarshal([]byte(gstr.Raw), &model)
	return model, err
}

func (m *appManageService) PutAppStatus(storeId string, status string) (bool, error) {
	url := strings.TrimSuffix(m.address, "/") + APIComposeStatus + "/" + storeId + "/status"

	body := []byte(`"` + status + `"`)
	response, err := http2.Put(url, body, 5*time.Second)
	if err != nil {
		return false, err
	}
	if response.StatusCode != http.StatusOK {
		return false, errors.New("failed to change status (status code: " + fmt.Sprint(response.StatusCode) + ")")
	}
	return true, nil
}

// NewAppManageService resolves the app-management URL from
// app-management.url under RuntimePath, retries up to ~10s for the
// file to appear during co-startup, pings /ping for readiness,
// and returns a ready-to-use AppManageService.
func NewAppManageService(RuntimePath string) (AppManageService, error) {
	managementAddressFile := filepath.Join(RuntimePath, AppManageURLFilename)

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

	if err := ping(address, 5*time.Second); err != nil {
		return nil, err
	}

	return &appManageService{
		address: address,
	}, nil
}
