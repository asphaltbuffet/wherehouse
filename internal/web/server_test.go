package web_test

import (
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/web"
)

func TestNew_DefaultsApplied(t *testing.T) {
	srv, err := web.New(web.Config{App: &fakeApp{}, Output: io.Discard})
	require.NoError(t, err)
	assert.NotNil(t, srv)
}

func TestRun_ShutdownOnContextCancel(t *testing.T) {
	srv, err := web.New(web.Config{
		App:    &fakeApp{},
		Bind:   "127.0.0.1",
		Port:   18080,
		Output: io.Discard,
	})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Run(ctx) }()

	cancel()
	err = <-errCh
	assert.NoError(t, err)
}
