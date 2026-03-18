package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newRemovedCommand(use, replacement string) *cobra.Command {
	return &cobra.Command{
		Use:                use,
		Hidden:             true,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			invoked := strings.TrimSpace(cmd.CommandPath())
			return usageError(fmt.Sprintf("%s was removed; use `certyn %s`", invoked, replacement), nil)
		},
	}
}

func newRemovedCommandWithMessage(use, message string) *cobra.Command {
	return &cobra.Command{
		Use:                use,
		Hidden:             true,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return usageError(message, nil)
		},
	}
}

func newRemovedCICommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "ci",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return usageError("certyn ci was removed; use `certyn run`", nil)
		},
	}
	cmd.AddCommand(newRemovedCommand("run", "run"))
	cmd.AddCommand(newRemovedCommand("status", "run status"))
	cmd.AddCommand(newRemovedCommand("wait", "run wait"))
	cmd.AddCommand(newRemovedCommand("cancel", "run cancel"))
	cmd.AddCommand(newRemovedCommand("list", "run list"))
	return cmd
}

func newRemovedVerifyCommand() *cobra.Command {
	return newRemovedCommandWithMessage(
		"verify",
		"certyn verify was removed; use `certyn run --url ...` or `certyn run --environment ...`",
	)
}
