package external

import (
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	http2 "github.com/neochaotic/powerlab/backend/common/utils/http"
)

const (
	APICasaOSShare = "/v1/samba/shares"
)

// ShareService is the SMB-share admin API surface — used to
// remove a share when the user deletes the underlying folder.
// Only Delete is exposed today; create/list still go through the
// CasaOS samba route directly.
type ShareService interface {
	// DeleteShare removes the SMB share with the given id from the
	// CasaOS samba config.
	DeleteShare(id string) error
}
type shareService struct {
	addressFile string
}

func (n *shareService) DeleteShare(id string) error {
	address, err := getAddress(n.addressFile)
	if err != nil {
		return err
	}

	url := strings.TrimSuffix(address, "/") + APICasaOSShare + "/" + id
	fmt.Println(url)

	response, err := http2.Delete(url, []byte("{}"), 30*time.Second)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return errors.New("failed to send share (status code: " + fmt.Sprint(response.StatusCode) + ")")
	}
	return nil
}

// NewShareService returns a ShareService bound to the casaos
// service URL resolved from runtimePath. Address resolution is
// lazy.
func NewShareService(runtimePath string) ShareService {
	return &shareService{
		addressFile: filepath.Join(runtimePath, CasaOSURLFilename),
	}
}
