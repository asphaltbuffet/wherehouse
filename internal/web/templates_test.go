package web_test

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

const (
	htmxConfigMeta   = `<meta name="htmx-config" content='{"includeIndicatorStyles":false}'>`
	htmxScriptTag    = `<script src="/static/htmx.min.js" defer></script>`
	htmxConfigMarker = `name="htmx-config"`
	mainTitle        = "<title>Wherehouse</title>"
	searchTitle      = "<title>Wherehouse — Search</title>"
)

func TestFullPages_DisableHTMXIndicatorStyles(t *testing.T) {
	fake := &fakeApp{
		entities: []app.EntityResult{
			{EntityID: "abc", DisplayName: "Garage", CanonicalName: "garage",
				FullPathDisplay: "Garage", Status: inventory.EntityStatusOk},
		},
		history: []app.HistoryResult{
			{EventID: 1, TimestampUTC: "2025-01-01T00:00:00Z", ActorUserID: "alice"},
		},
	}

	tests := []struct {
		name  string
		fake  *fakeApp
		path  string
		title string
	}{
		{name: "index", fake: fake, path: "/", title: mainTitle},
		{name: "detail", fake: fake, path: "/entities/abc", title: mainTitle},
		{name: "search", fake: fake, path: "/search?q=garage", title: searchTitle},
		{name: "search-empty", fake: &fakeApp{}, path: "/search?q=nomatch", title: searchTitle},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := newTestServer(t, tt.fake)
			defer ts.Close()

			resp, err := ts.Client().Get(ts.URL + tt.path)
			require.NoError(t, err)
			defer resp.Body.Close()

			require.Equal(t, http.StatusOK, resp.StatusCode)
			raw, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			body := string(raw)

			require.Contains(t, body, htmxConfigMeta)
			assert.Less(t, strings.Index(body, htmxConfigMeta), strings.Index(body, htmxScriptTag),
				"htmx-config meta must precede the htmx script so htmx sees it at init")
			assert.Contains(t, body, tt.title)
		})
	}
}

func TestHTMXFragment_OmitsHead(t *testing.T) {
	ts := newTestServer(t, &fakeApp{})
	defer ts.Close()

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/search?q=garage", nil)
	require.NoError(t, err)
	req.Header.Set("Hx-Request", "true")

	resp, err := ts.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), htmxConfigMarker)
}
