package common

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/ini.v1"

	"github.com/neochaotic/powerlab/backend/common/utils/constants"
)

const (
	ConfigKeyLogPath     = "gateway.LogPath"
	ConfigKeyLogSaveName = "gateway.LogSaveName"
	ConfigKeyLogFileExt  = "gateway.LogFileExt"
	ConfigKeyGatewayPort = "gateway.Port"
	ConfigKeyRuntimePath = "common.RuntimePath"

	GatewayName       = "gateway"
	GatewayConfigType = "ini"
)

// Config is the gateway's minimal ini-backed configuration. It reads
// `section.Key` values from gateway.ini and persists the port back when
// the runtime picks a free one.
//
// It deliberately does NOT use a config framework. The previous viper
// dependency dropped its built-in ini codec in v1.20, so a routine
// v1.21 bump made the gateway panic at boot on every fresh install
// ("decoder not found for this format") — a failure no build or unit
// test caught because nothing exercised the real parse path. The
// gateway only reads two keys, so gopkg.in/ini.v1 directly is both
// lighter and unbreakable-by-codec-removal.
type Config struct {
	file     *ini.File
	path     string
	defaults map[string]string
}

func gatewayDefaults() map[string]string {
	return map[string]string{
		ConfigKeyLogPath:     constants.DefaultLogPath,
		ConfigKeyLogSaveName: GatewayName,
		ConfigKeyLogFileExt:  "log",
		// See https://refspecs.linuxfoundation.org/FHS_3.0/fhs/ch05s13.html
		ConfigKeyRuntimePath: constants.DefaultRuntimePath,
	}
}

// splitKey splits a "section.Key" config key. A key with no dot maps to
// the ini default (unnamed) section.
func splitKey(dotted string) (section, key string) {
	if i := strings.Index(dotted, "."); i >= 0 {
		return dotted[:i], dotted[i+1:]
	}
	return "", dotted
}

// GetString returns the value for a "section.Key", falling back to the
// registered default, then to "". Never panics on a missing key.
func (c *Config) GetString(dotted string) string {
	section, key := splitKey(dotted)
	if c.file != nil {
		if s, err := c.file.GetSection(section); err == nil && s.HasKey(key) {
			return s.Key(key).String()
		}
	}
	return c.defaults[dotted]
}

// Set assigns a value in-memory; persist it with WriteConfig.
func (c *Config) Set(dotted, value string) {
	if c.file == nil {
		c.file = ini.Empty()
	}
	section, key := splitKey(dotted)
	c.file.Section(section).Key(key).SetValue(value)
}

// WriteConfig saves the in-memory config back to the file it was loaded
// from.
func (c *Config) WriteConfig() error {
	if c.path == "" {
		return fmt.Errorf("no config file path to write to")
	}
	return c.file.SaveTo(c.path)
}

// loadConfigFrom loads a specific gateway.ini path and applies defaults.
func loadConfigFrom(path string) (*Config, error) {
	f, err := ini.Load(path)
	if err != nil {
		return nil, fmt.Errorf("load %s: %w", path, err)
	}
	return &Config{file: f, path: path, defaults: gatewayDefaults()}, nil
}

// LoadConfig finds gateway.ini in the standard PowerLab search path
// (cwd, cwd/conf, $CASAOS_CONFIG_PATH, the packaged config dir) and
// loads it. Missing keys fall back to constants.* defaults.
func LoadConfig() (*Config, error) {
	var searchPaths []string
	if cwd, err := os.Getwd(); err != nil {
		log.Println(err)
	} else {
		searchPaths = append(searchPaths, cwd, filepath.Join(cwd, "conf"))
	}
	if envPath, ok := os.LookupEnv("CASAOS_CONFIG_PATH"); ok {
		searchPaths = append(searchPaths, envPath)
	}
	searchPaths = append(searchPaths, constants.DefaultConfigPath)

	fileName := GatewayName + "." + GatewayConfigType
	for _, dir := range searchPaths {
		if dir == "" {
			continue
		}
		candidate := filepath.Join(dir, fileName)
		if _, err := os.Stat(candidate); err == nil {
			return loadConfigFrom(candidate)
		}
	}
	return nil, fmt.Errorf("config file %q not found in search paths %v", fileName, searchPaths)
}
