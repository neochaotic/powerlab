package op

import (
	"regexp"
	"strings"

	jsoniter "github.com/json-iterator/go"
	"github.com/neochaotic/powerlab/backend/common/utils/logger"
	"github.com/neochaotic/powerlab/backend/core/internal/conf"
	"github.com/neochaotic/powerlab/backend/core/internal/driver"
	"github.com/neochaotic/powerlab/backend/core/model"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

// ObjsUpdateHook fires after a directory listing returns — used by
// search-index updaters and per-driver caches.
type ObjsUpdateHook = func(parent string, objs []model.Obj)

// ObjsUpdateHooks is the package-level hook list.
var (
	ObjsUpdateHooks = make([]ObjsUpdateHook, 0)
)

// RegisterObjsUpdateHook appends hook to the fan-out list.
func RegisterObjsUpdateHook(hook ObjsUpdateHook) {
	ObjsUpdateHooks = append(ObjsUpdateHooks, hook)
}

// HandleObjsUpdateHook fans out a list-result to every registered
// ObjsUpdateHook.
func HandleObjsUpdateHook(parent string, objs []model.Obj) {
	for _, hook := range ObjsUpdateHooks {
		hook(parent, objs)
	}
}

// SettingItemHook fires after a setting row is mutated — used by
// settings whose values cache derived state (e.g. file-extension
// type lists, regex compilations).
type SettingItemHook func(item *model.SettingItem) error

var settingItemHooks = map[string]SettingItemHook{
	conf.VideoTypes: func(item *model.SettingItem) error {
		conf.SlicesMap[conf.VideoTypes] = strings.Split(item.Value, ",")
		return nil
	},
	conf.AudioTypes: func(item *model.SettingItem) error {
		conf.SlicesMap[conf.AudioTypes] = strings.Split(item.Value, ",")
		return nil
	},
	conf.ImageTypes: func(item *model.SettingItem) error {
		conf.SlicesMap[conf.ImageTypes] = strings.Split(item.Value, ",")
		return nil
	},
	conf.TextTypes: func(item *model.SettingItem) error {
		conf.SlicesMap[conf.TextTypes] = strings.Split(item.Value, ",")
		return nil
	},
	conf.ProxyTypes: func(item *model.SettingItem) error {
		conf.SlicesMap[conf.ProxyTypes] = strings.Split(item.Value, ",")
		return nil
	},
	conf.ProxyIgnoreHeaders: func(item *model.SettingItem) error {
		conf.SlicesMap[conf.ProxyIgnoreHeaders] = strings.Split(item.Value, ",")
		return nil
	},
	conf.PrivacyRegs: func(item *model.SettingItem) error {
		regStrs := strings.Split(item.Value, "\n")
		regs := make([]*regexp.Regexp, 0, len(regStrs))
		for _, regStr := range regStrs {
			reg, err := regexp.Compile(regStr)
			if err != nil {
				return errors.WithStack(err)
			}
			regs = append(regs, reg)
		}
		conf.PrivacyReg = regs
		return nil
	},
	conf.FilenameCharMapping: func(item *model.SettingItem) error {
		var json = jsoniter.ConfigCompatibleWithStandardLibrary
		err := json.UnmarshalFromString(item.Value, &conf.FilenameCharMap)
		if err != nil {
			return err
		}
		logger.Info("filename char mapping", zap.Any("FilenameCharMap", conf.FilenameCharMap))
		return nil
	},
}

// RegisterSettingItemHook registers hook to fire when the setting
// identified by key changes.
func RegisterSettingItemHook(key string, hook SettingItemHook) {
	settingItemHooks[key] = hook
}

// HandleSettingItemHook invokes the hook (if any) registered for
// item.Key. hasHook reports whether a hook was found + invoked.
func HandleSettingItemHook(item *model.SettingItem) (hasHook bool, err error) {
	if hook, ok := settingItemHooks[item.Key]; ok {
		return true, hook(item)
	}
	return false, nil
}

// StorageHook fires when a storage backend is added/removed/
// updated. typ is one of "add", "remove", "update".
type StorageHook func(typ string, storage driver.Driver)

var storageHooks = make([]StorageHook, 0)

// CallStorageHooks fans out a storage event to every registered
// StorageHook.
func CallStorageHooks(typ string, storage driver.Driver) {
	for _, hook := range storageHooks {
		hook(typ, storage)
	}
}

// RegisterStorageHook appends hook to the fan-out list.
func RegisterStorageHook(hook StorageHook) {
	storageHooks = append(storageHooks, hook)
}
