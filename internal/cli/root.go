package cli

import "github.com/spf13/cobra"

func NewRootCommand() *cobra.Command {
	flags := &GlobalFlags{}
	app := NewApp(flags)

	root := &cobra.Command{
		Use:           "certyn",
		Short:         "Certyn CLI",
		Version:       Version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.PersistentFlags().StringVar(&flags.APIURL, "api-url", "", "Certyn API URL (default resolved from config/env)")
	root.PersistentFlags().StringVar(&flags.APIKey, "api-key", "", "Certyn API key (default resolved from config/env)")
	root.PersistentFlags().StringVar(&flags.Profile, "profile", "", "Config profile name")
	root.PersistentFlags().StringVar(&flags.Project, "project", "", "Default project slug override")
	root.PersistentFlags().StringVar(&flags.Environment, "environment", "", "Default environment key/id override")
	root.PersistentFlags().BoolVar(&flags.JSON, "json", false, "Emit JSON output")

	root.AddCommand(newCICommand(app))
	root.AddCommand(newRunnersCommand(app))
	root.AddCommand(newProjectsCommand(app))
	root.AddCommand(newEnvCommand(app))
	root.AddCommand(newTestcasesCommand(app))
	root.AddCommand(newObservationsCommand(app))
	root.AddCommand(newIssuesCommand(app))
	root.AddCommand(newExecutionsCommand(app))
	root.AddCommand(newAskCommand(app))
	root.AddCommand(newVerifyCommand(app))
	root.AddCommand(newConfigCommand(app))

	return root
}
