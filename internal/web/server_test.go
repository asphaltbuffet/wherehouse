package web_test

import (
	"bytes"
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
		Port:   0,
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

func TestRun_ActorHeaderBindWarning(t *testing.T) {
	cases := []struct {
		name   string
		bind   string
		header string
		warn   bool
	}{
		{"header on all interfaces", "0.0.0.0", "Tailscale-User-Login", true},
		{"header on loopback", "127.0.0.1", "Tailscale-User-Login", false},
		{"no header on all interfaces", "0.0.0.0", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stderr bytes.Buffer
			srv, err := web.New(web.Config{
				App:         &fakeApp{},
				Bind:        tc.bind,
				Port:        0,
				Output:      io.Discard,
				ErrOutput:   &stderr,
				ActorHeader: tc.header,
			})
			require.NoError(t, err)

			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			require.NoError(t, srv.Run(ctx))

			if tc.warn {
				assert.Contains(t, stderr.String(), "warning: trusting identity header Tailscale-User-Login")
				assert.Contains(t, stderr.String(), tc.bind)
				return
			}
			assert.Empty(t, stderr.String())
		})
	}
}
