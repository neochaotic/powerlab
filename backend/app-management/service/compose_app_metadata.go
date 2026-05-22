package service

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/compose-spec/compose-go/v2/loader"
	"github.com/neochaotic/powerlab/backend/app-management/codegen"
	"github.com/neochaotic/powerlab/backend/app-management/common"
	"github.com/neochaotic/powerlab/backend/common/utils/logger"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

// StoreInfo extracts the catalog metadata (icon, screenshots,
// title, port-map, tags, etc.) from the compose file's
// x-powerlab/x-casaos extension block. includeApps true also
// resolves the per-service StoreInfo of each App in the project.
func (a *ComposeApp) StoreInfo(includeApps bool) (*codegen.ComposeAppStoreInfo, error) {
	ex, ok := a.getExtension()
	if !ok {
		return nil, ErrComposeExtensionNotFound
	}

	var storeInfo codegen.ComposeAppStoreInfo
	if err := loader.Transform(ex, &storeInfo); err != nil {
		logger.Error("Transform store info fail", zap.Error(err))
		return nil, err
	}

	// Aliases: map 'web' or 'port' to 'port_map' if 'port_map' is empty.
	// PowerLab store YAMLs use 'web:' for clarity; CasaOS uses 'port_map:' or 'port:'.
	if storeInfo.PortMap == "" {
		if extMap, ok := ex.(map[string]interface{}); ok {
			for _, key := range []string{"web", "port"} {
				if v, ok := extMap[key].(string); ok && v != "" {
					storeInfo.PortMap = v
					break
				} else if v, ok := extMap[key].(int); ok {
					storeInfo.PortMap = strconv.Itoa(v)
					break
				}
			}
		}
	}

	// TODO refactor this with ComposeAppWithStoreInfo
	if extMap, ok := a.getExtensionMap(); ok {
		if val, ok := extMap[common.ComposeExtensionPropertyNameIsUncontrolled]; ok {
			if isUncontrolled, ok := val.(bool); ok {
				storeInfo.IsUncontrolled = &isUncontrolled
			}
		}
	}

	// locate main app
	if storeInfo.Main == nil || *storeInfo.Main == "" {
		// if main app is not specified, use the first app
		for _, app := range a.Apps() {
			storeInfo.Main = &app.Name
			break
		}
	}

	if storeInfo.Scheme == nil || *storeInfo.Scheme == "" {
		storeInfo.Scheme = lo.ToPtr(codegen.Http)
	}

	if includeApps {
		apps := map[string]codegen.AppStoreInfo{}

		for _, app := range a.Apps() {
			appStoreInfo, err := app.StoreInfo()
			if err != nil {
				if err == ErrComposeExtensionNotFound {
					logger.Info("App does not have x-casaos extension - skipping", zap.String("app", app.Name))
					continue
				}

				return nil, err
			}
			apps[app.Name] = appStoreInfo
		}

		storeInfo.Apps = &apps
	}

	return &storeInfo, nil
}

// getExtension returns the compose extension regardless of which alias the
// author used (x-powerlab, x-web, or x-casaos). See service/extension.go.
func (a *ComposeApp) getExtension() (interface{}, bool) {
	v, _, ok := LookupAppExtension(a.Extensions)
	return v, ok
}

// getExtensionMap returns the extension as a map regardless of which alias
// the author used. See service/extension.go.
func (a *ComposeApp) getExtensionMap() (map[string]interface{}, bool) {
	m, _, ok := LookupAppExtensionMap(a.Extensions)
	return m, ok
}

// AuthorType reports whether the app is from the official PowerLab
// catalog, an editor-curated submission, or a community-author
// submission — drives the badge on the catalog detail page.
func (a *ComposeApp) AuthorType() codegen.StoreAppAuthorType {
	storeInfo, err := a.StoreInfo(false)
	if err != nil {
		return codegen.Unknown
	}

	if strings.EqualFold(storeInfo.Author, storeInfo.Developer) {
		return codegen.Official
	}
	if strings.EqualFold(storeInfo.Author, common.ComposeAppAuthorCasaOSTeam) {
		return codegen.ByCasaos
	}

	return codegen.Community
}

// SetStoreAppID writes the store-app id into the compose file's
// x-extension block. Returns (existingID, isStoreApp) — bool false
// means the compose file has no x-extension at all (custom app).
func (a *ComposeApp) SetStoreAppID(storeAppID string) (string, bool) {
	// set store_app_id (by convention is the same as app name at install time if it does not exist)
	composeAppStoreInfo, ok := a.getExtensionMap()
	if !ok {
		logger.Info("compose app does not have a valid extension - might not be a PowerLab app", zap.String("app", a.Name))
		return "", false
	}

	value, ok := composeAppStoreInfo[common.ComposeExtensionPropertyNameStoreAppID]
	if ok {
		currentStoreAppID, ok := value.(string)
		if ok {
			logger.Info("compose app already has store_app_id", zap.String("app", a.Name), zap.String("storeAppID", currentStoreAppID))
			return currentStoreAppID, true
		}
	}

	composeAppStoreInfo[common.ComposeExtensionPropertyNameStoreAppID] = storeAppID
	return storeAppID, true
}

// SetTitle writes a localised title into the x-extension block.
// lang is a BCP-47 code; "en" is the default.
func (a *ComposeApp) SetTitle(title, lang string) {
	if a.Extensions == nil {
		a.Extensions = make(map[string]interface{})
	}

	composeAppStoreInfo, ok := a.getExtensionMap()
	if !ok {
		// Create a new extension using the preferred key
		composeAppStoreInfo = map[string]interface{}{}
		a.Extensions[common.ComposeExtensionNameWeb] = composeAppStoreInfo
	}

	if _, ok := composeAppStoreInfo[common.ComposeExtensionPropertyNameTitle]; !ok {
		composeAppStoreInfo[common.ComposeExtensionPropertyNameTitle] = map[string]string{}
	}

	titleMap, ok := composeAppStoreInfo[common.ComposeExtensionPropertyNameTitle].(map[string]string)
	if !ok {
		logger.Info("compose app does not have valid title map in its extension", zap.String("app", a.Name))
		return
	}

	if _, ok := titleMap[lang]; !ok {
		titleMap[lang] = title
	}
}

// UpdateEventPropertiesFromStoreInfo tries to update AppIcon and
// AppTitle in the given event properties map from the compose
// app's store-info block. Used by lifecycle handlers to enrich
// message-bus events with display metadata.
func (a *ComposeApp) UpdateEventPropertiesFromStoreInfo(eventProperties map[string]string) error {
	if eventProperties == nil {
		return fmt.Errorf("event properties is nil")
	}

	storeInfo, err := a.StoreInfo(false)
	if err != nil {
		return err
	}

	eventProperties[common.PropertyTypeAppIcon.Name] = storeInfo.Icon

	if storeInfo.Title == nil {
		return fmt.Errorf("compose app title not found in store info")
	}

	titles, err := json.Marshal(storeInfo.Title)
	if err != nil {
		return err
	}

	eventProperties[common.PropertyTypeAppTitle.Name] = string(titles)

	return nil
}

// SetUncontrolled marks the compose app as user-managed (true) or
// PowerLab-managed (false). Stored in the x-extension block so it
// survives compose-file rewrites.
func (a *ComposeApp) SetUncontrolled(uncontrolled bool) error {
	extMap, ok := a.getExtensionMap()
	if !ok {
		logger.Error("failed to get extension map", zap.String("composeAppID", a.Name))
		return ErrComposeExtensionNotFound
	}

	extMap[common.ComposeExtensionPropertyNameIsUncontrolled] = uncontrolled
	return nil
}
