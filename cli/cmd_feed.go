package cli

import (
	"github.com/spf13/cobra"
)

// feedSectionCmd returns a named command that reads a specific Bloomberg feed section.
func (a *App) feedSectionCmd(name, short, section string) *cobra.Command {
	return &cobra.Command{
		Use:   name,
		Short: short,
		RunE: func(cmd *cobra.Command, _ []string) error {
			n := a.effectiveLimit(20)
			a.progressf("fetching bloomberg %s...", section)
			articles, err := a.client.Feed(cmd.Context(), section, n)
			if err != nil {
				return mapFetchErr(err)
			}
			return a.renderOrEmpty(articles, len(articles))
		},
	}
}

// feedCmd returns the generic `feed <section>` command.
func (a *App) feedCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "feed <section>",
		Short: "Read any Bloomberg feed section by slug",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			section := args[0]
			n := a.effectiveLimit(20)
			a.progressf("fetching bloomberg %s feed...", section)
			articles, err := a.client.Feed(cmd.Context(), section, n)
			if err != nil {
				return mapFetchErr(err)
			}
			return a.renderOrEmpty(articles, len(articles))
		},
	}
}

// sectionsCmd returns the `sections` command listing all known feed sections.
func (a *App) sectionsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sections",
		Short: "List all available Bloomberg feed sections",
		RunE: func(cmd *cobra.Command, _ []string) error {
			sections := a.client.Sections()
			return a.renderOrEmpty(sections, len(sections))
		},
	}
}
