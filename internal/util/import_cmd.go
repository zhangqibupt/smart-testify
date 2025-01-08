package util

import (
	"os"
	"os/exec"
)

func RunGoImports(fileName string) error {
	cmd := exec.Command("goimports", "-w", fileName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
