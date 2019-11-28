package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "dhammer",
	Short: "Load-testing and benchmarking tool.",
	Long:  `Dhammer is a load-testing and benchmarking tool.  Originally designed for DHCP, it's now growing into a more general purpose tool.`,
}

// Execute executes the root command.
func Execute() error {
	return rootCmd.Execute()
}
