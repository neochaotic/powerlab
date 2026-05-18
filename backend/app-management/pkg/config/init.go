package config

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/neochaotic/powerlab/backend/app-management/common"
	"github.com/neochaotic/powerlab/backend/app-management/model"
	"github.com/neochaotic/powerlab/backend/common/utils/constants"
	"github.com/neochaotic/powerlab/backend/common/utils/devmode"
	"gopkg.in/ini.v1"
)

var (
	CommonInfo = &model.CommonModel{
		RuntimePath: constants.DefaultRuntimePath,
	}

	AppInfo = &model.APPModel{
		AppStorePath: filepath.Join(constants.DefaultDataPath, "appstore"),
		AppsPath:     filepath.Join(constants.DefaultDataPath, "apps"),
		LogPath:      constants.DefaultLogPath,
		LogSaveName:  common.AppManagementServiceName,
		LogFileExt:   "log",
	}

	ServerInfo = &model.ServerModel{
		AppStoreList: []string{},
	}

	// Global is a map to inject environment variables to the app.
	Global = make(map[string]string)

	AppLifecycleFlags = &model.AppLifecycleFlags{}

	Cfg               *ini.File
	ConfigFilePath    string
	GlobalEnvFilePath string
)

func ReloadConfig() {
	var err error
	Cfg, err = ini.LoadSources(ini.LoadOptions{Insensitive: true, AllowShadows: true}, ConfigFilePath)
	if err != nil {
		fmt.Println("failed to reload config", err)
	} else {
		mapTo("common", CommonInfo)
		mapTo("app", AppInfo)
		mapTo("server", ServerInfo)
	}
}

func InitSetup(config string, sample string) {
	ConfigFilePath = AppManagementConfigFilePath
	if len(config) > 0 {
		ConfigFilePath = config
	}

	// create default config file if not exist
	if _, err := os.Stat(ConfigFilePath); os.IsNotExist(err) {
		fmt.Println("config file not exist, create it")
		// create config file
		file, err := os.Create(ConfigFilePath)
		if err != nil {
			panic(err)
		}
		defer file.Close()

		// write default config
		_, err = file.WriteString(sample)
		if err != nil {
			panic(err)
		}
	}

	var err error

	Cfg, err = ini.LoadSources(ini.LoadOptions{Insensitive: true, AllowShadows: true}, ConfigFilePath)
	if err != nil {
		panic(err)
	}

	mapTo("common", CommonInfo)
	mapTo("app", AppInfo)
	mapTo("server", ServerInfo)

	// ADR-0038: prune legacy catalog sources from the loaded config.
	// Operators upgrading from <v0.7.0 may still have CasaOS jsdelivr
	// / big-bear URLs in their on-disk conf; this migration strips
	// them and persists the cleaned list. Idempotent — clean configs
	// are left untouched.
	MigrateAppStoreListLegacyRemoval()

	// ADR-0039 (#450) — remove orphaned community-catalog/Apps/ dirs.
	// Boxes upgraded from a PowerLab version that shipped a larger
	// curated set may have leftover app dirs on disk that the v0.7.0
	// install.sh `cp -R` overlay never removed. Compares disk vs the
	// `.curated-manifest` file written into the catalog dir by
	// scripts/package-linux.sh and removes anything not in the
	// manifest. Missing/empty manifest → noop (safer to leave orphans
	// than wipe operator state). The install-time wipe-then-copy from
	// PR #451 is the authoritative path for fresh installs; this
	// migration is the safety net for v0.7.0 boxes that already have
	// orphans on disk + future-proof against any path that bypasses
	// install.sh.
	MigrateOrphanCuratedApps()

	// #437 (Sprint 23) — tag apps installed before v0.7.1 with a
	// .installed-pre-v0.7.1 marker. The follow-up UI work will consume
	// this marker to surface a "Legacy" badge in the apps grid.
	// Idempotent via the .legacy-scan-complete sentinel under
	// AppsPath — runs once on the first v0.7.1 boot, then never again.
	// New installs (post-v0.7.1) never get the marker.
	MigrateLegacyAppMarker()

	// Dev sandbox: when there is no production install (/etc/powerlab),
	// redirect runtime + app data into the project tree so multiple
	// services can share a writable sandbox under `./start.sh`. In
	// production trust whatever was loaded from /etc/powerlab/*.conf.
	if devmode.IsDev() {
		if currentDir, err := os.Getwd(); err == nil {
			sharedRuntime := filepath.Join(filepath.Dir(currentDir), "runtime")
			CommonInfo.RuntimePath = sharedRuntime
			AppInfo.LogPath = filepath.Join(filepath.Dir(sharedRuntime), "logs")
			AppInfo.AppStorePath = filepath.Join(filepath.Dir(sharedRuntime), "data", "appstore")
			AppInfo.AppsPath = filepath.Join(filepath.Dir(sharedRuntime), "data", "apps")
		}
	}

	// StoragePath is the root for app volume bind-mounts (e.g. /DATA on
	// Linux). macOS cannot create root-level directories (SIP) so we
	// pivot to /tmp/powerlab-data which Docker Desktop shares by default.
	// This rule is OS-level, not dev-vs-prod, so it runs in both modes.
	if AppInfo.StoragePath == "" || AppInfo.StoragePath == "/DATA" {
		if runtime.GOOS == "darwin" {
			AppInfo.StoragePath = "/tmp/powerlab-data"
		} else {
			AppInfo.StoragePath = "/DATA"
		}
	}
}

func SaveSetup() error {
	reflectFrom("common", CommonInfo)
	reflectFrom("app", AppInfo)
	reflectFrom("server", ServerInfo)

	return Cfg.SaveTo(ConfigFilePath)
}

func InitGlobal(config string) {
	// read file
	// file content like this:
	// OPENAI_API_KEY=123456

	// read file
	GlobalEnvFilePath = AppManagementGlobalEnvFilePath
	if len(config) > 0 {
		ConfigFilePath = config
	}

	// from file read key and value
	// set to Global
	file, err := os.Open(GlobalEnvFilePath)
	// there can't to panic err. because the env file is a new file
	// very much user didn't have the file.
	if err != nil {
		// log.Fatal will exit the program. So we only can to log the error.
		log.Println("open global env file error:", err)
	} else {
		defer file.Close()
		scanner := bufio.NewScanner(file)
		scanner.Split(bufio.ScanLines)

		for scanner.Scan() {
			line := scanner.Text()
			parts := strings.Split(line, "=")
			Global[parts[0]] = parts[1]
		}
	}
}

func SaveGlobal() error {
	// file content like this:
	// OPENAI_API_KEY=123456
	file, err := os.Create(AppManagementGlobalEnvFilePath)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)

	for key, value := range Global {
		fmt.Fprintf(writer, "%s=%s\n", key, value)
	}

	writer.Flush()
	return err
}

func mapTo(section string, v interface{}) {
	err := Cfg.Section(section).MapTo(v)
	if err != nil {
		log.Fatalf("Cfg.MapTo %s err: %v", section, err)
	}
}

func reflectFrom(section string, v interface{}) {
	err := Cfg.Section(section).ReflectFrom(v)
	if err != nil {
		log.Fatalf("Cfg.ReflectFrom %s err: %v", section, err)
	}
}
