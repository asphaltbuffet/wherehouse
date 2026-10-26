package serve_test

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/cmd/serve"
	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/config"
)

type fakeApp struct{}

func (f *fakeApp) ListEntities(_ context.Context) ([]app.EntityResult, error) { return nil, nil }
func (f *fakeApp) GetEntityByID(_ context.Context, _ string) (app.EntityResult, error) {
	return app.EntityResult{}, nil
}
func (f *fakeApp) GetChildren(_ context.Context, _ string) ([]app.EntityResult, error) {
	return nil, nil
}
func (f *fakeApp) HasLiveChildren(_ context.Context, _ string) (bool, error) { return false, nil }
func (f *fakeApp) GetHistory(_ context.Context, _ app.GetHistoryRequest) ([]app.HistoryResult, error) {
	return nil, nil
}

func (f *fakeApp) CreateEntity(_ context.Context, _ app.CreateEntityRequest) (app.EntityResult, error) {
	return app.EntityResult{}, nil
}

func (f *fakeApp) RenameEntity(_ context.Context, _ app.RenameEntityRequest) (app.EntityResult, error) {
	return app.EntityResult{}, nil
}

func (f *fakeApp) MarkLost(_ context.Context, _ []app.ChangeStatusRequest) ([]app.EntityResult, error) {
	return nil, nil
}

func (f *fakeApp) MarkFound(_ context.Context, _ []app.ChangeStatusRequest) ([]app.EntityResult, error) {
	return nil, nil
}

func (f *fakeApp) FindEntities(_ context.Context, _ app.FindEntitiesRequest) ([]app.FindResult, error) {
	return nil, nil
}

func (f *fakeApp) MarkReturned(_ context.Context, _ []app.ChangeStatusRequest) ([]app.EntityResult, error) {
	return nil, nil
}

func (f *fakeApp) MarkLoaned(_ context.Context, _ []app.ChangeStatusRequest) ([]app.EntityResult, error) {
	return nil, nil
}

func (f *fakeApp) FindParentCandidates(_ context.Context, _ app.FindEntitiesRequest) ([]app.FindResult, error) {
	return nil, nil
}

func (f *fakeApp) GetEntityByIDAnyStatus(_ context.Context, _ string) (app.EntityResult, error) {
	return app.EntityResult{}, nil
}

func (f *fakeApp) BorrowEntities(_ context.Context, _ []app.BorrowEntityRequest) ([]app.EntityResult, error) {
	return nil, nil
}

func (f *fakeApp) SetAttributes(_ context.Context, _ []app.SetAttributesRequest) ([]app.EntityResult, error) {
	return nil, nil
}

func TestBuildServeCmd_FlagDefaults(t *testing.T) {
	cmd := serve.NewServeCmd(&fakeApp{})

	port, err := cmd.Flags().GetInt("port")
	require.NoError(t, err)
	assert.Equal(t, 8080, port)

	bind, err := cmd.Flags().GetString("bind")
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1", bind)
}

func TestBuildServeCmd_FlagOverride(t *testing.T) {
	cmd := serve.NewServeCmd(&fakeApp{})

	err := cmd.ParseFlags([]string{"--port", "9090", "--bind", "0.0.0.0"})
	require.NoError(t, err)

	port, _ := cmd.Flags().GetInt("port")
	bind, _ := cmd.Flags().GetString("bind")
	assert.Equal(t, 9090, port)
	assert.Equal(t, "0.0.0.0", bind)
}

func TestBuildServeCmd_ActorHeaderDefault(t *testing.T) {
	cmd := serve.NewServeCmd(&fakeApp{})

	h, err := cmd.Flags().GetString("actor-header")
	require.NoError(t, err)
	assert.Empty(t, h)
}

// runServe executes serve on 0.0.0.0 with an already-cancelled context so it
// starts and shuts down immediately. The non-loopback warning on stderr
// reveals which actor header won.
func runServe(t *testing.T, cfgHeader string, args ...string) (string, error) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if cfgHeader != "" {
		cfg := config.GetDefaults()
		cfg.Web.ActorHeader = cfgHeader
		ctx = context.WithValue(ctx, config.ConfigKey, cfg)
	}

	var stderr bytes.Buffer
	cmd := serve.NewServeCmd(&fakeApp{})
	cmd.SetOut(io.Discard)
	cmd.SetErr(&stderr)
	cmd.SetArgs(append([]string{"--bind", "0.0.0.0", "--port", "0"}, args...))
	err := cmd.ExecuteContext(ctx)
	return stderr.String(), err
}

func TestServe_ActorHeaderPrecedence(t *testing.T) {
	cases := []struct {
		name      string
		cfgHeader string
		args      []string
		want      string // header named in the warning; "" means no warning
	}{
		{"neither", "", nil, ""},
		{"config only", "X-Config-User", nil, "X-Config-User"},
		{"flag only", "", []string{"--actor-header", "Tailscale-User-Login"}, "Tailscale-User-Login"},
		{
			"flag beats config", "X-Config-User",
			[]string{"--actor-header", "Tailscale-User-Login"}, "Tailscale-User-Login",
		},
		{"empty flag disables config", "X-Config-User", []string{"--actor-header", ""}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stderr, err := runServe(t, tc.cfgHeader, tc.args...)
			require.NoError(t, err)
			if tc.want == "" {
				assert.NotContains(t, stderr, "warning: trusting identity header")
				return
			}
			assert.Contains(t, stderr, "warning: trusting identity header "+tc.want)
		})
	}
}

func TestServe_InvalidActorHeader(t *testing.T) {
	_, err := runServe(t, "", "--actor-header", "Tailscale User")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid actor header name")
}
