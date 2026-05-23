package serve_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/cmd/serve"
	"github.com/asphaltbuffet/wherehouse/internal/app"
)

type fakeApp struct{}

func (f *fakeApp) ListEntities(_ context.Context) ([]app.EntityResult, error) { return nil, nil }
func (f *fakeApp) GetChildren(_ context.Context, _ string) ([]app.EntityResult, error) {
	return nil, nil
}
func (f *fakeApp) GetEntityByPath(_ context.Context, _ string) (app.EntityResult, error) {
	return app.EntityResult{}, nil
}
func (f *fakeApp) GetHistory(_ context.Context, _ app.GetHistoryRequest) ([]app.HistoryResult, error) {
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
