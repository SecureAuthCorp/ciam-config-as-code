package cmd

import "github.com/spf13/cobra"

func mustMarkRequired(cmd *cobra.Command, flags ...string) {
	for _, flag := range flags {
		err := cmd.MarkFlagRequired(flag)

		if err != nil {
			panic(err)
		}
	}
}
