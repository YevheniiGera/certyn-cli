package cli

import (
	"fmt"
	"time"

	"github.com/certyn/certyn-cli/internal/api"
	"github.com/certyn/certyn-cli/internal/config"
	"github.com/spf13/cobra"
)

func newRunnersCommand(app *App) *cobra.Command {
	runnersCmd := &cobra.Command{
		Use:   "runners",
		Short: "Self-hosted runner operations",
	}

	poolsCmd := &cobra.Command{Use: "pools", Short: "Runner pool operations"}
	poolsCmd.AddCommand(newRunnerPoolsListCommand(app))
	poolsCmd.AddCommand(newRunnerPoolsCreateCommand(app))
	poolsCmd.AddCommand(newRunnerPoolsDeleteCommand(app))

	tokensCmd := &cobra.Command{Use: "tokens", Short: "Runner registration token operations"}
	tokensCmd.AddCommand(newRunnerTokensCreateCommand(app))

	runnersCmd.AddCommand(poolsCmd)
	runnersCmd.AddCommand(tokensCmd)
	runnersCmd.AddCommand(newRunnersListCommand(app))
	runnersCmd.AddCommand(newRunnersDrainCommand(app))
	runnersCmd.AddCommand(newRunnersResumeCommand(app))

	return runnersCmd
}

func newRunnerPoolsListCommand(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List runner pools",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, client, printer, err := app.ResolveRuntime(config.ResolveInput{}, true)
			if err != nil {
				return err
			}
			pools, err := client.ListRunnerPools(cmd.Context())
			if err != nil {
				return classifyAPIError(err, "failed to list runner pools")
			}
			if printer.JSON {
				return printer.EmitJSON(pools)
			}
			for _, pool := range pools {
				fmt.Printf("- %s name=%s kind=%s active=%t max=%d slots=%d\n", pool.ID, pool.Name, pool.PoolKind, pool.IsActive, pool.MaxRunners, pool.SlotsPerRunner)
			}
			return nil
		},
	}
}

func newRunnerPoolsCreateCommand(app *App) *cobra.Command {
	var name string
	var description string
	var minRunners int
	var maxRunners int
	var slotsPerRunner int
	var cloudRegion string
	var scaleSetResourceID string
	var shared bool

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a runner pool",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireValue("name", name); err != nil {
				return err
			}
			_, client, printer, err := app.ResolveRuntime(config.ResolveInput{}, true)
			if err != nil {
				return err
			}

			input := api.CreateRunnerPoolRequest{
				Name:               name,
				Description:        description,
				IsShared:           shared,
				CloudRegion:        cloudRegion,
				ScaleSetResourceID: scaleSetResourceID,
			}
			if cmd.Flags().Changed("min-runners") {
				input.MinRunners = intPtr(minRunners)
			}
			if cmd.Flags().Changed("max-runners") {
				input.MaxRunners = intPtr(maxRunners)
			}
			if cmd.Flags().Changed("slots-per-runner") {
				input.SlotsPerRunner = intPtr(slotsPerRunner)
			}
			pool, err := client.CreateRunnerPool(cmd.Context(), input)
			if err != nil {
				return classifyAPIError(err, "failed to create runner pool")
			}

			if printer.JSON {
				return printer.EmitJSON(pool)
			}
			fmt.Printf("Created runner pool %s (%s)\n", pool.ID, pool.Name)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Pool name")
	cmd.Flags().StringVar(&description, "description", "", "Pool description")
	cmd.Flags().IntVar(&minRunners, "min-runners", 0, "Minimum runners")
	cmd.Flags().IntVar(&maxRunners, "max-runners", 0, "Maximum runners")
	cmd.Flags().IntVar(&slotsPerRunner, "slots-per-runner", 1, "Slots per runner")
	cmd.Flags().StringVar(&cloudRegion, "cloud-region", "", "Cloud region")
	cmd.Flags().StringVar(&scaleSetResourceID, "scale-set-resource-id", "", "Scale set resource ID")
	cmd.Flags().BoolVar(&shared, "shared", false, "Mark pool as shared")

	return cmd
}

func newRunnerPoolsDeleteCommand(app *App) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "delete <pool-id>",
		Short: "Delete a runner pool",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				return usageError("pass --yes to confirm pool deletion", nil)
			}
			_, client, printer, err := app.ResolveRuntime(config.ResolveInput{}, true)
			if err != nil {
				return err
			}

			if err := client.DeleteRunnerPool(cmd.Context(), args[0]); err != nil {
				return classifyAPIError(err, "failed to delete runner pool")
			}
			if printer.JSON {
				return printer.EmitJSON(map[string]any{
					"pool_id": args[0],
					"deleted": true,
				})
			}
			fmt.Printf("Deleted runner pool %s\n", args[0])
			return nil
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "Confirm pool deletion")
	return cmd
}

func newRunnerTokensCreateCommand(app *App) *cobra.Command {
	var poolID string
	var mode string
	var maxUses int
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a runner registration token",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireValue("pool", poolID); err != nil {
				return err
			}
			_, client, printer, err := app.ResolveRuntime(config.ResolveInput{}, true)
			if err != nil {
				return err
			}

			input := api.CreateRunnerTokenRequest{TokenMode: mode}
			if maxUses > 0 {
				input.MaxUses = intPtr(maxUses)
			}

			resp, err := client.CreateRunnerToken(cmd.Context(), poolID, input)
			if err != nil {
				return classifyAPIError(err, "failed to create runner token")
			}

			if printer.JSON {
				return printer.EmitJSON(resp)
			}
			fmt.Printf("Token created for pool %s\n", resp.PoolID)
			fmt.Printf("Mode: %s\n", resp.TokenMode)
			fmt.Printf("Expires: %s\n", resp.ExpiresAtUTC.Format(time.RFC3339))
			fmt.Printf("Token: %s\n", resp.Token)
			return nil
		},
	}

	cmd.Flags().StringVar(&poolID, "pool", "", "Runner pool ID")
	cmd.Flags().StringVar(&mode, "mode", "", "Token mode: single_use or multi_use")
	cmd.Flags().IntVar(&maxUses, "max-uses", 0, "Maximum token uses for multi-use tokens")

	return cmd
}

func newRunnersListCommand(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List self-hosted runners",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, client, printer, err := app.ResolveRuntime(config.ResolveInput{}, true)
			if err != nil {
				return err
			}
			runners, err := client.ListRunners(cmd.Context())
			if err != nil {
				return classifyAPIError(err, "failed to list runners")
			}
			if printer.JSON {
				return printer.EmitJSON(runners)
			}
			for _, runner := range runners {
				fmt.Printf("- %s name=%s status=%s pool=%s slots=%d/%d\n", runner.ID, runner.Name, runner.Status, runner.PoolID, runner.AvailableSlots, runner.MaxConcurrency)
			}
			return nil
		},
	}
}

func newRunnersDrainCommand(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "drain <runner-id>",
		Short: "Drain a runner",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, client, printer, err := app.ResolveRuntime(config.ResolveInput{}, true)
			if err != nil {
				return err
			}
			if err := client.DrainRunner(cmd.Context(), args[0]); err != nil {
				return classifyAPIError(err, "failed to drain runner")
			}
			if printer.JSON {
				return printer.EmitJSON(map[string]any{
					"runner_id": args[0],
					"drained":   true,
				})
			}
			fmt.Printf("Runner %s set to draining\n", args[0])
			return nil
		},
	}
}

func newRunnersResumeCommand(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "resume <runner-id>",
		Short: "Resume a runner",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, client, printer, err := app.ResolveRuntime(config.ResolveInput{}, true)
			if err != nil {
				return err
			}
			if err := client.ResumeRunner(cmd.Context(), args[0]); err != nil {
				return classifyAPIError(err, "failed to resume runner")
			}
			if printer.JSON {
				return printer.EmitJSON(map[string]any{
					"runner_id": args[0],
					"resumed":   true,
				})
			}
			fmt.Printf("Runner %s resumed\n", args[0])
			return nil
		},
	}
}

func intPtr(value int) *int {
	return &value
}
