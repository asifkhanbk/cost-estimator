package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	planFile       string
	providerFilter string
)

// rootCmd is the main CLI entrypoint
var rootCmd = &cobra.Command{
	Use:   "cost-estimator",
	Short: "Azure Infra Cost Estimator",
	Long:  "Parse a Terraform JSON plan and output a precise monthly cost breakdown using Azure Retail Prices API.",
}

// Execute runs the CLI
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&planFile, "plan", "p", "", "Path to Terraform JSON plan")
	rootCmd.PersistentFlags().StringVar(&providerFilter, "provider", "", "Filter by provider prefix (e.g., azurerm)")
	// No need to add estimateCmd here, as estimateCmd.go calls AddCommand
}