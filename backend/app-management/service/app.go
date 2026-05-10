package service

import (
	"github.com/compose-spec/compose-go/loader"
	"github.com/compose-spec/compose-go/types"
	"github.com/neochaotic/powerlab/backend/app-management/codegen"
	"github.com/neochaotic/powerlab/backend/common/utils/logger"
)

// App is the in-process view of one service inside a compose file.
// Type-aliased to compose-spec's ServiceConfig so PowerLab's
// extension methods (StoreInfo + helpers) can hang off the same
// value the compose loader produces.
type App types.ServiceConfig

// StoreInfo extracts the PowerLab/CasaOS x-extension block from
// the service config — the catalog metadata (icon, description,
// screenshots, port map) that the compose-author embeds via
// `x-powerlab:` (or legacy `x-web` / `x-casaos`).
func (a *App) StoreInfo() (codegen.AppStoreInfo, error) {
	var storeInfo codegen.AppStoreInfo

	ex, _, ok := LookupAppExtension(a.Extensions)
	if !ok {
		logger.Error("PowerLab/CasaOS extension not found (tried x-powerlab, x-web, x-casaos)")
	}

	// add image to store info for check stable version function.
	storeInfo.Image = a.Image

	if err := loader.Transform(ex, &storeInfo); err != nil {
		return storeInfo, err
	}

	return storeInfo, nil
}
