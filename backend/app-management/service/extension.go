package service

import "github.com/neochaotic/powerlab/backend/app-management/common"

// extensionPriority is the order in which we look up the PowerLab/CasaOS
// extension on a compose document. The first key that resolves to a non-nil
// value wins. Order matters:
//
//  1. x-powerlab — PowerLab canonical (authored by our UI)
//  2. x-web      — intermediate alias previously used upstream
//  3. x-casaos   — original CasaOS extension; what most store apps ship with
var extensionPriority = []string{
	common.ComposeExtensionNameXPowerLab,
	common.ComposeExtensionNameWeb,
	common.ComposeExtensionNameXCasaOS,
}

// LookupAppExtension returns the first present extension on a compose
// document or service, regardless of which alias the author used. The
// returned `key` is the actual map key the value was found under — call
// sites that mutate the extension should write back to this same key
// to preserve the original author's choice.
func LookupAppExtension(extensions map[string]interface{}) (value interface{}, key string, found bool) {
	if extensions == nil {
		return nil, "", false
	}
	for _, k := range extensionPriority {
		if v, ok := extensions[k]; ok && v != nil {
			return v, k, true
		}
	}
	return nil, "", false
}

// LookupAppExtensionMap is the same as LookupAppExtension but pre-asserts
// the value to map[string]interface{}, which is what almost every reader
// actually wants.
func LookupAppExtensionMap(extensions map[string]interface{}) (m map[string]interface{}, key string, found bool) {
	v, k, ok := LookupAppExtension(extensions)
	if !ok {
		return nil, "", false
	}
	m, ok = v.(map[string]interface{})
	if !ok {
		return nil, "", false
	}
	return m, k, true
}
