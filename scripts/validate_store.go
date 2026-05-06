package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type CasaOSInfo struct {
	Title       map[string]string `yaml:"title"`
	Icon        string            `yaml:"icon"`
	Description map[string]string `yaml:"description"`
	Developer   string            `yaml:"developer"`
	Author      string            `yaml:"author"`
	Category    string            `yaml:"category"`
	Tagline     map[string]string `yaml:"tagline"`
	Web         string            `yaml:"web"`
	Main        string            `yaml:"main"`
}

type ComposeFile struct {
	Name     string                 `yaml:"name"`
	Services map[string]interface{} `yaml:"services"`
	XCasaOS  CasaOSInfo             `yaml:"x-casaos"`
}

func main() {
	fmt.Println("🔍 Validating PowerLab App Store...")

	storePath := "store/Apps"
	apps, err := ioutil.ReadDir(storePath)
	if err != nil {
		fmt.Printf("❌ Failed to read store directory: %v\n", err)
		os.Exit(1)
	}

	failed := false
	client := &http.Client{Timeout: 5 * time.Second}

	for _, app := range apps {
		if !app.IsDir() {
			continue
		}

		appID := app.Name()
		composePath := filepath.Join(storePath, appID, "docker-compose.yml")

		if _, err := os.Stat(composePath); os.IsNotExist(err) {
			fmt.Printf("❌ [%s] Missing docker-compose.yml\n", appID)
			failed = true
			continue
		}

		data, err := ioutil.ReadFile(composePath)
		if err != nil {
			fmt.Printf("❌ [%s] Failed to read compose file: %v\n", appID)
			failed = true
			continue
		}

		var compose ComposeFile
		if err := yaml.Unmarshal(data, &compose); err != nil {
			fmt.Printf("❌ [%s] YAML syntax error: %v\n", appID)
			failed = true
			continue
		}

		// Validate x-casaos extension
		if err := validateXCasaOS(appID, compose.XCasaOS, client); err != nil {
			fmt.Printf("❌ [%s] Validation failed: %v\n", appID, err)
			failed = true
			continue
		}

		// Validate service names
		if compose.XCasaOS.Main != "" {
			if _, ok := compose.Services[compose.XCasaOS.Main]; !ok {
				fmt.Printf("❌ [%s] 'main' service '%s' not found in services list\n", appID, compose.XCasaOS.Main)
				failed = true
				continue
			}
		}

		fmt.Printf("✅ [%s] Valid\n", appID)
	}

	if failed {
		fmt.Println("\n🏁 Store validation FAILED.")
		os.Exit(1)
	}

	fmt.Println("\n🏁 Store validation PASSED.")
}

func validateXCasaOS(id string, info CasaOSInfo, client *http.Client) error {
	if info.Title["en_us"] == "" {
		return fmt.Errorf("missing 'title.en_us'")
	}
	if info.Icon == "" {
		return fmt.Errorf("missing 'icon' URL")
	}
	if info.Developer == "" {
		return fmt.Errorf("missing 'developer'")
	}
	if info.Author == "" {
		return fmt.Errorf("missing 'author'")
	}
	if info.Category == "" {
		return fmt.Errorf("missing 'category'")
	}
	if info.Main == "" {
		return fmt.Errorf("missing 'main' service identification")
	}

	// Validate Icon URL
	if strings.HasPrefix(info.Icon, "http") {
		resp, err := client.Head(info.Icon)
		if err != nil {
			fmt.Printf("⚠️  [%s] Icon URL unreachable: %v\n", id, err)
		} else if resp.StatusCode != http.StatusOK {
			fmt.Printf("⚠️  [%s] Icon URL returned status %d\n", id, resp.StatusCode)
		}
		if resp != nil {
			resp.Body.Close()
		}
	}

	return nil
}
