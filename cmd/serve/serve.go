package serve

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/web"
)

// NewDefaultServeCmd returns the serve command wired to the real database.
func NewDefaultServeCmd() *cobra.Command {
	cmd := buildServeCmd()
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		s, a, err := cli.OpenDatabase(cmd.Context())
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer s.Close()
		return runServe(cmd, a)
	}
	return cmd
}

// NewServeCmd returns the serve command with an injected app (for tests).
func NewServeCmd(a web.App) *cobra.Command {
	cmd := buildServeCmd()
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		return runServe(cmd, a)
	}
	return cmd
}

const defaultPort = 8080

func buildServeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start a local web server to browse the inventory",
		Long: `Start a local HTTP server that renders the inventory as a navigable
tree in your browser. You can browse, search, add entities, edit names, and
toggle item status (active/missing) from the UI.

Examples:
  wherehouse serve                   # Listen on 127.0.0.1:8080
  wherehouse serve --port 9090
  wherehouse serve --bind 0.0.0.0   # Listen on all interfaces (LAN)`,
		Args: cobra.NoArgs,
	}
	cmd.Flags().IntP("port", "p", defaultPort, "port to listen on")
	cmd.Flags().String("bind", "127.0.0.1", "address to bind to")
	return cmd
}

func runServe(cmd *cobra.Command, a web.App) error {
	port, err := cmd.Flags().GetInt("port")
	if err != nil {
		return fmt.Errorf("get port flag: %w", err)
	}
	bind, err := cmd.Flags().GetString("bind")
	if err != nil {
		return fmt.Errorf("get bind flag: %w", err)
	}

	srv, err := web.New(web.Config{
		App:    a,
		Bind:   bind,
		Port:   port,
		Output: cmd.OutOrStdout(),
	})
	if err != nil {
		return fmt.Errorf("create server: %w", err)
	}
	return srv.Run(cmd.Context())
}
