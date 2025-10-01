package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var helloWorldCmd = &cobra.Command{
	Use:   "hello-world",
	Short: "Print a friendly greeting",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprintln(cmd.OutOrStdout(), "Hello from the DocumentDB kubectl plugin!")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(helloWorldCmd)
}
