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

func TestCSRF_RejectsPOSTWithoutHxRequest(t *testing.T) {
	entities := []app.EntityResult{
		{EntityID: "abc", DisplayName: "Hammer", FullPathDisplay: "Garage:Hammer",
			EntityType: inventory.EntityTypeLeaf, Status: inventory.EntityStatusOk},
	}
	ts := newTestServer(t, &fakeApp{entities: entities})
	defer ts.Close()

	form := strings.NewReader("display_name=Nope")
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/entities/abc/edit/name", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := ts.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestCSRF_RejectsCrossOriginPOST(t *testing.T) {
	entities := []app.EntityResult{
		{EntityID: "abc", DisplayName: "Hammer", FullPathDisplay: "Garage:Hammer",
			EntityType: inventory.EntityTypeLeaf, Status: inventory.EntityStatusOk},
	}
	ts := newTestServer(t, &fakeApp{entities: entities})
	defer ts.Close()

	form := strings.NewReader("display_name=Nope")
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/entities/abc/edit/name", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Hx-Request", "true")
	req.Header.Set("Origin", "http://evil.example.com")
	resp, err := ts.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestSecurityHeaders_Present(t *testing.T) {
	ts := newTestServer(t, &fakeApp{})
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/healthz")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.NotEmpty(t, resp.Header.Get("Content-Security-Policy"))
	assert.Equal(t, "nosniff", resp.Header.Get("X-Content-Type-Options"))
	assert.Equal(t, "no-referrer", resp.Header.Get("Referrer-Policy"))
}

func TestSearch_NonHTMXRendersFullShell(t *testing.T) {
	entities := []app.EntityResult{
		{EntityID: "abc", DisplayName: "Garage", CanonicalName: "garage",
			FullPathDisplay: "Garage",
			EntityType:      inventory.EntityTypePlace, Status: inventory.EntityStatusOk},
	}
	ts := newTestServer(t, &fakeApp{entities: entities})
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/search?q=foo")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	bs := string(body)
	assert.Contains(t, bs, "<!doctype html>")
	assert.Contains(t, bs, "tree-root-list")
	assert.Contains(t, bs, "Garage")
}

func TestSearch_RejectsOverlyLongQuery(t *testing.T) {
	ts := newTestServer(t, &fakeApp{})
	defer ts.Close()

	long := strings.Repeat("a", 200)
	resp, err := ts.Client().Get(ts.URL + "/search?q=" + long)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestLimitBody_RejectsOversizedPOST(t *testing.T) {
	entities := []app.EntityResult{
		{EntityID: "abc", DisplayName: "Hammer", FullPathDisplay: "Garage:Hammer",
			EntityType: inventory.EntityTypeLeaf, Status: inventory.EntityStatusOk},
	}
	ts := newTestServer(t, &fakeApp{entities: entities})
	defer ts.Close()

	// 32KiB > maxFormBytes (16KiB).
	big := "display_name=" + strings.Repeat("A", 32*1024)
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/entities/abc/edit/name", strings.NewReader(big))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Hx-Request", "true")
	resp, err := ts.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// MaxBytesReader makes ParseForm return an error; handler responds 400.
	assert.NotEqual(t, http.StatusOK, resp.StatusCode)
	_, _ = io.Copy(io.Discard, resp.Body)
}
