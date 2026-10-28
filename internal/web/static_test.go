package web_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStaticFonts(t *testing.T) {
	ts := newTestServer(t, &fakeApp{})
	defer ts.Close()

	tests := []struct {
		path string
		want int
	}{
		{"/static/fonts/Barlow-Regular.woff2", http.StatusOK},
		{"/static/fonts/Barlow-Medium.woff2", http.StatusOK},
		{"/static/fonts/Barlow-SemiBold.woff2", http.StatusOK},
		{"/static/fonts/BarlowCondensed-Bold.woff2", http.StatusOK},
		{"/static/fonts/SpaceMono-Regular.woff2", http.StatusOK},
		{"/static/fonts/SpaceGrotesk-Regular.woff2", http.StatusNotFound},
	}
	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			resp, err := ts.Client().Get(ts.URL + tc.path)
			require.NoError(t, err)
			defer resp.Body.Close()
			assert.Equal(t, tc.want, resp.StatusCode)
		})
	}
}

func TestStaticCSS_NoSpaceGrotesk(t *testing.T) {
	ts := newTestServer(t, &fakeApp{})
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/static/app.css")
	require.NoError(t, err)
	defer resp.Body.Close()
	body := readBody(t, resp)

	assert.Contains(t, body, `font-family: "Barlow"`)
	assert.Contains(t, body, `font-family: "Barlow Condensed"`)
	assert.NotContains(t, body, "Space Grotesk")
}
