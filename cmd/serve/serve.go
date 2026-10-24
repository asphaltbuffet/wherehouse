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

Identity: web changes are recorded as actor "webui". With --actor-header
(or web.actor_header / WHEREHOUSE_WEB_ACTOR_HEADER), the named request
header's value is recorded instead.

wherehouse does NOT authenticate requests. Only set an actor header behind
a reverse proxy that overwrites that header and is the sole path to this
listener (e.g. --bind 127.0.0.1 inside a Tailscale sidecar's network
namespace). Anyone who can reach the listener directly can forge it.

Examples:
  wherehouse serve                   # Listen on 127.0.0.1:8080
  wherehouse serve --port 9090
  wherehouse serve --bind 0.0.0.0   # Listen on all interfaces (LAN)
  wherehouse serve --actor-header Tailscale-User-Login`,
		Args: cobra.NoArgs,
	}
	cmd.Flags().IntP("port", "p", defaultPort, "port to listen on")
	cmd.Flags().String("bind", "127.0.0.1", "address to bind to")
	cmd.Flags().String("actor-header", "",
		"trusted proxy header recorded as the actor (overrides web.actor_header)")
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

	actorHeader, err := resolveActorHeader(cmd)
	if err != nil {
		return err
	}

	srv, err := web.New(web.Config{
		App:         a,
		Bind:        bind,
		Port:        port,
		Output:      cmd.OutOrStdout(),
		ErrOutput:   cmd.ErrOrStderr(),
		ActorHeader: actorHeader,
	})
	if err != nil {
		return fmt.Errorf("create server: %w", err)
	}
	return srv.Run(cmd.Context())
}

// resolveActorHeader applies --actor-header > web.actor_header (config file
// or WHEREHOUSE_WEB_ACTOR_HEADER). An explicitly empty flag disables it.
func resolveActorHeader(cmd *cobra.Command) (string, error) {
	if cmd.Flags().Changed("actor-header") {
		h, err := cmd.Flags().GetString("actor-header")
		if err != nil {
			return "", fmt.Errorf("get actor-header flag: %w", err)
		}
		return h, nil
	}
	if cfg, ok := cli.GetConfig(cmd.Context()); ok {
		return cfg.Web.ActorHeader, nil
	}
	return "", nil
}
