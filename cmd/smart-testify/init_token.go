package main

import (
	"github.com/spf13/cobra"
	"log"
	"smart-testify/internal/copilot"
)

// initTokenCmd initializes the Copilot token and saves it
var initTokenCmd = &cobra.Command{
	Use:   "init-token",
	Short: "Initialize Copilot token and save it to ~/.copilot_token",
	Run: func(cmd *cobra.Command, args []string) {
		if err := copilot.NewCopilotClient().GenerateToken(); err != nil {
			log.Fatalf("Error initializing token: %v", err)
		}
	},
}
