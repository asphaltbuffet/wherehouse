package web_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
	"github.com/asphaltbuffet/wherehouse/internal/web"
)

const tsHeader = "Tailscale-User-Login"

func hammer() []app.EntityResult {
	return []app.EntityResult{
		{EntityID: "abc", DisplayName: "Hammer", FullPathDisplay: "Garage:Hammer",
			Status: inventory.EntityStatusOk},
	}
}

func postForm(target, body string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Hx-Request", "true")
	return req
}

func TestActor_RecordedOnEveryMutation(t *testing.T) {
	cases := []struct {
		name string
		path string
		body string
	}{
		{"create root", "/entities/add", "display_name=Wrench"},
		{"create child", "/entities/abc/add", "display_name=Wrench"},
		{"rename", "/entities/abc/edit/name", "display_name=Mallet"},
		{"toggle missing", "/entities/abc/actions/toggle-missing", ""},
		{"set attribute", "/entities/abc/actions/attr", "locked=true"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fa := &fakeApp{entities: hammer(), renameResult: hammer()[0]}
			h := newActorHandler(t, fa, tsHeader)

			req := postForm(tc.path, tc.body)
			req.Header.Set(tsHeader, "alice@example.com")
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			assert.Equal(t, "alice@example.com", fa.lastActor)
		})
	}
}

func TestActor_Resolution(t *testing.T) {
	cases := []struct {
		name       string
		configured string // web.Config.ActorHeader
		sent       map[string]string
		body       string
		want       string
	}{
		{"off, no header", "", nil, "", "webui"},
		{"off, header sent", "", map[string]string{tsHeader: "mallory"}, "", "webui"},
		{"on, header absent", tsHeader, nil, "", "webui"},
		{"on, header blank", tsHeader, map[string]string{tsHeader: "   "}, "", "webui"},
		{"on, header present", tsHeader, map[string]string{tsHeader: "alice@example.com"}, "", "alice@example.com"},
		{"on, header padded", tsHeader, map[string]string{tsHeader: "  alice@example.com\t"}, "", "alice@example.com"},
		{
			"lowercase config name", "tailscale-user-login",
			map[string]string{tsHeader: "alice@example.com"}, "", "alice@example.com",
		},
		{
			"padded config name", " " + tsHeader + " \t",
			map[string]string{tsHeader: "alice@example.com"}, "", "alice@example.com",
		},
		{"form user ignored when off", "", nil, "user=mallory", "webui"},
		{
			"form user ignored when on", tsHeader,
			map[string]string{tsHeader: "alice@example.com"}, "user=mallory", "alice@example.com",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fa := &fakeApp{}
			h := newActorHandler(t, fa, tc.configured)

			body := "display_name=Wrench"
			if tc.body != "" {
				body += "&" + tc.body
			}
			req := postForm("/entities/add", body)
			for k, v := range tc.sent {
				req.Header.Set(k, v)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			assert.Equal(t, tc.want, fa.lastActor)
		})
	}
}

func TestNew_ActorHeaderValidation(t *testing.T) {
	cases := []struct {
		header  string
		wantErr bool
	}{
		{"", false},
		{"Tailscale-User-Login", false},
		{"X-Remote_User", false},
		{"Tailscale User", true},
		{"X:Y", true},
		{"X\nY", true},
		{"Ünicode", true},
	}
	for _, tc := range cases {
		t.Run(tc.header, func(t *testing.T) {
			_, err := web.New(web.Config{App: &fakeApp{}, Output: io.Discard, ActorHeader: tc.header})
			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "invalid actor header name")
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestAddItemForm_HasNoUserField(t *testing.T) {
	h := newActorHandler(t, &fakeApp{}, "")
	req := httptest.NewRequest(http.MethodGet, "/entities/add", nil)
	req.Header.Set("Hx-Request", "true")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.NotContains(t, rec.Body.String(), `name="user"`)
}

func getPage(t *testing.T, h http.Handler, target string, hdr map[string]string) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return rec.Body.String()
}

func TestHeader_ShowsActorOnEveryFullPage(t *testing.T) {
	for _, page := range []string{"/", "/search?q=ham", "/entities/abc"} {
		t.Run(page, func(t *testing.T) {
			h := newActorHandler(t, &fakeApp{entities: hammer()}, tsHeader)
			body := getPage(t, h, page, map[string]string{tsHeader: "alice@example.com"})

			assert.Contains(t, body, "acting as alice@example.com")
			assert.Contains(t, body, `title="from header Tailscale-User-Login"`)
		})
	}
}

func TestHeader_ActorSource(t *testing.T) {
	cases := []struct {
		name       string
		configured string
		sent       map[string]string
		wantText   string
		wantTitle  string
	}{
		{"not configured", "", nil, "acting as webui", `title="no identity header configured"`},
		{"configured, absent", tsHeader, nil, "acting as webui", `title="header Tailscale-User-Login not received"`},
		{"configured lowercase, present", "tailscale-user-login",
			map[string]string{tsHeader: "alice@example.com"},
			"acting as alice@example.com", `title="from header Tailscale-User-Login"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newActorHandler(t, &fakeApp{entities: hammer()}, tc.configured)
			body := getPage(t, h, "/", tc.sent)

			assert.Contains(t, body, tc.wantText)
			assert.Contains(t, body, tc.wantTitle)
			assert.NotContains(t, body, "signed in")
		})
	}
}

func TestHeader_EscapesActor(t *testing.T) {
	h := newActorHandler(t, &fakeApp{entities: hammer()}, tsHeader)
	body := getPage(t, h, "/", map[string]string{tsHeader: `<script>alert(1)</script>`})

	assert.NotContains(t, body, "<script>alert(1)</script>")
	assert.Contains(t, body, "acting as &lt;script&gt;alert(1)&lt;/script&gt;")
}
