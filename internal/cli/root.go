package cli

import "github.com/spf13/cobra"

const (
	commandGroupCore     = "core"
	commandGroupAdvanced = "advanced"
	commandGroupUtility  = "utility"
)

func NewRootCommand() *cobra.Command {
	flags := &GlobalFlags{}
	app := NewApp(flags)

	root := &cobra.Command{
		Use:   "certyn",
		Short: "Ask questions, run validations, and diagnose failures",
		Long: `Certyn CLI

Core workflow:
  certyn ask "What should I check before changing the login flow?"
  certyn login
  certyn whoami
  certyn doctor
  certyn init
  certyn run smoke --project my-app --environment staging --wait
  certyn diagnose --project my-app <execution-id>

Profile management:
  certyn config show
  certyn update
  certyn uninstall

Advanced operator commands remain available under:
  projects, environments, issues, tests, observations, executions, runners`,
		Version:       Version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddGroup(
		&cobra.Group{ID: commandGroupCore, Title: "Core Commands"},
		&cobra.Group{ID: commandGroupAdvanced, Title: "Advanced Commands"},
		&cobra.Group{ID: commandGroupUtility, Title: "Utility Commands"},
	)
	root.SetHelpCommandGroupID(commandGroupUtility)
	root.SetCompletionCommandGroupID(commandGroupUtility)

	root.PersistentFlags().StringVar(&flags.APIURL, "api-url", "", "Certyn API URL (default resolved from config/env)")
	root.PersistentFlags().StringVar(&flags.APIKey, "api-key", "", "Certyn API key (default resolved from config/env)")
	root.PersistentFlags().StringVar(&flags.Profile, "profile", "", "Config profile name")
	root.PersistentFlags().StringVar(&flags.Project, "project", "", "Default project slug override")
	root.PersistentFlags().StringVar(&flags.Environment, "environment", "", "Default environment key/id override")
	root.PersistentFlags().BoolVar(&flags.JSON, "json", false, "Emit JSON output")

	loginCmd := newLoginCommand(app)
	loginCmd.GroupID = commandGroupCore
	logoutCmd := newLogoutCommand(app)
	logoutCmd.GroupID = commandGroupCore
	whoAmICmd := newWhoAmICommand(app)
	whoAmICmd.GroupID = commandGroupCore
	doctorCmd := newDoctorCommand(app)
	doctorCmd.GroupID = commandGroupCore
	initCmd := newInitCommand(app)
	initCmd.GroupID = commandGroupCore
	askCmd := newAskCommand(app)
	askCmd.GroupID = commandGroupCore
	updateCmd := newUpdateCommand(app)
	updateCmd.GroupID = commandGroupCore
	uninstallCmd := newUninstallCommand(app)
	uninstallCmd.GroupID = commandGroupCore
	runCmd := newRunCommand(app)
	runCmd.GroupID = commandGroupCore
	diagnoseCmd := newDiagnoseCommand(app)
	diagnoseCmd.GroupID = commandGroupCore
	configCmd := newConfigCommand(app)
	configCmd.GroupID = commandGroupCore

	projectsCmd := newProjectsCommand(app)
	projectsCmd.GroupID = commandGroupAdvanced
	environmentsCmd := newEnvironmentsCommand(app)
	environmentsCmd.GroupID = commandGroupAdvanced
	issuesCmd := newIssuesCommand(app)
	issuesCmd.GroupID = commandGroupAdvanced
	testsCmd := newTestsCommand(app)
	testsCmd.GroupID = commandGroupAdvanced
	observationsCmd := newObservationsCommand(app)
	observationsCmd.GroupID = commandGroupAdvanced
	executionsCmd := newExecutionsCommand(app)
	executionsCmd.GroupID = commandGroupAdvanced
	runnersCmd := newRunnersCommand(app)
	runnersCmd.GroupID = commandGroupAdvanced

	root.AddCommand(askCmd)
	root.AddCommand(loginCmd)
	root.AddCommand(logoutCmd)
	root.AddCommand(whoAmICmd)
	root.AddCommand(doctorCmd)
	root.AddCommand(initCmd)
	root.AddCommand(runCmd)
	root.AddCommand(diagnoseCmd)
	root.AddCommand(configCmd)
	root.AddCommand(updateCmd)
	root.AddCommand(uninstallCmd)
	root.AddCommand(projectsCmd)
	root.AddCommand(environmentsCmd)
	root.AddCommand(issuesCmd)
	root.AddCommand(testsCmd)
	root.AddCommand(observationsCmd)
	root.AddCommand(executionsCmd)
	root.AddCommand(runnersCmd)
	root.AddCommand(newRemovedCommand("env", "environments"))
	root.AddCommand(newRemovedCommand("testcases", "tests"))
	root.AddCommand(newRemovedCommand("tickets", "issues"))
	root.AddCommand(newRemovedVerifyCommand())
	root.AddCommand(newRemovedCICommand())

	return root
}
