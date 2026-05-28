package man

import (
	"fmt"

	mcobra "github.com/muesli/mango-cobra"
	"github.com/muesli/roff"
	"github.com/spf13/cobra"
)

// NewManCmd returns the hidden "man" subcommand that writes roff man page content to stdout.
func NewManCmd() *cobra.Command {
	cmd := buildManCmd()
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		return runMan(cmd)
	}
	return cmd
}

func buildManCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "man",
		Short:  "Generate man page",
		Hidden: true,
		Args:   cobra.NoArgs,
	}
}

func runMan(cmd *cobra.Command) error {
	manPage, err := mcobra.NewManPage(1, cmd.Root())
	if err != nil {
		return fmt.Errorf("generate man page: %w", err)
	}
	_, err = fmt.Fprint(cmd.OutOrStdout(), manPage.Build(roff.NewDocument()))
	return err
}
