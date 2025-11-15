package main

import (
	"smart-testify/internal/copilot"

	"github.com/spf13/cobra"
)

var copilotClient *copilot.Client

func getCopilotClient() *copilot.Client {
	if copilotClient == nil {
		copilotClient = copilot.NewCopilotClient(getGlobalConfig().CopilotToken, false)
	}

	return copilotClient
}

var initTokenCmd = &cobra.Command{
	Use:   "init-token",
	Short: "Initialize Copilot token",
	Run: func(cmd *cobra.Command, args []string) {
		if err := InitCopilotToken(); err != nil {
			log.Errorf("Failed to initialize Copilot token: %v", err)
		}
	},
}

var copilotCmd = &cobra.Command{
	Use:   "copilot",
	Short: "Configure settings for Copilot.",
}

func init() {
	copilotCmd.AddCommand(initTokenCmd)
}

func InitCopilotToken() error {
	config, err := loadConfig()
	if err != nil {
		return err
	}

	token, err := copilot.GetCopilotToken()
	if err != nil {
		return err
	}

	config.CopilotToken = token
	if err := saveConfig(config); err != nil {
		return err
	}

	log.Infof("Copilot token initialized successfully")
	return nil
}
