package web_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

func TestHandleHealthz(t *testing.T) {
	ts := newTestServer(t, &fakeApp{})
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/healthz")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// Responses that switch between a fragment and a full page on htmx headers must
// say so in Vary, or a cache can replay one shape for the other request (#296).
func TestFragmentOrPageResponses_VaryOnHTMXHeaders(t *testing.T) {
	ts := newTestServer(t, &fakeApp{entities: []app.EntityResult{
		{EntityID: "abc", DisplayName: "Garage", CanonicalName: "garage",
			FullPathDisplay: "Garage", Status: inventory.EntityStatusOk},
	}})
	defer ts.Close()

	paths := []string{"/", "/entities/abc", "/search?q=gar", "/entities/abc/actions/loan"}
	headerSets := map[string]map[string]string{
		"full page": {},
		"fragment":  {"Hx-Request": "true"},
		"restore":   {"Hx-Request": "true", "Hx-History-Restore-Request": "true"},
	}
	for _, path := range paths {
		for name, headers := range headerSets {
			t.Run(path+" "+name, func(t *testing.T) {
				req, err := http.NewRequest(http.MethodGet, ts.URL+path, nil)
				require.NoError(t, err)
				for k, v := range headers {
					req.Header.Set(k, v)
				}
				resp, err := ts.Client().Do(req)
				require.NoError(t, err)
				defer resp.Body.Close()

				require.Equal(t, http.StatusOK, resp.StatusCode)
				vary := resp.Header.Values("Vary")
				assert.Contains(t, vary, "Hx-Request")
				assert.Contains(t, vary, "Hx-History-Restore-Request")
			})
		}
	}
}
