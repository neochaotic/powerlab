package pkg

import (
	"github.com/IceWhaleTech/CasaOS-AppManagement/codegen"
	"github.com/IceWhaleTech/CasaOS-AppManagement/service"
	"github.com/compose-spec/compose-go/loader"
)

func VaildDockerCompose(yaml []byte) (err error) {
	err = nil
	// recover
	defer func() {
		if r := recover(); r != nil {
			err = r.(error)
		}
	}()
	docker, err := service.NewComposeAppFromYAML(yaml, false, false)

	// Translation layer: accept x-powerlab (canonical), x-web, or
	// x-casaos. See service/extension.go.
	ex, _, ok := service.LookupAppExtension(docker.Extensions)
	if !ok {
		return service.ErrComposeExtensionNameXCasaOSNotFound
	}

	var storeInfo codegen.ComposeAppStoreInfo
	if err = loader.Transform(ex, &storeInfo); err != nil {
		return
	}

	return
}
