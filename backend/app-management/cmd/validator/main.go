package main

import (
	"fmt"
	"os"

	"github.com/neochaotic/powerlab/backend/app-management/cmd/validator/pkg"
	"github.com/neochaotic/powerlab/backend/common/utils/file"
	utils_logger "github.com/neochaotic/powerlab/backend/common/utils/logger"
)

var logger = NewLogger()

func main() {
	utils_logger.LogInitConsoleOnly()

	if len(os.Args) < 1 {
		os.Args = append(os.Args, "-h")
	}

	dockerComposeFilePath := os.Args[1]

	// check file exists
	if _, err := os.Stat(dockerComposeFilePath); os.IsNotExist(err) {
		logger.Error("docker-compose file does not exist: %s", dockerComposeFilePath)
		os.Exit(1)
	}

	composeFileContent := file.ReadFullFile(dockerComposeFilePath)

	err := pkg.VaildDockerCompose(composeFileContent)
	if err != nil {
		logger.Error("failed to parse docker-compose file %s", err.Error())
		os.Exit(1)
	}
	fmt.Println("pass validate")
}
