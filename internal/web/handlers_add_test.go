package web_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

func TestAddItem_LockedDiscreteFlags(t *testing.T) {
	parent := app.EntityResult{
		EntityID: "g", DisplayName: "Garage", FullPathDisplay: "Garage",
		Status: inventory.EntityStatusOk,
	}
	cases := []struct {
		name         string
		path         string
		extra        string
		wantLocked   bool
		wantDiscrete bool
	}{
		{"root neither", "/entities/add", "", false, false},
		{"root locked", "/entities/add", "locked=on", true, false},
		{"root discrete", "/entities/add", "discrete=on", false, true},
		{"root both", "/entities/add", "locked=on&discrete=on", true, true},
		{"child neither", "/entities/g/add", "", false, false},
		{"child locked", "/entities/g/add", "locked=on", true, false},
		{"child discrete", "/entities/g/add", "discrete=on", false, true},
		{"child both", "/entities/g/add", "locked=on&discrete=on", true, true},
		{"empty value is false", "/entities/add", "locked=&discrete=", false, false},
		{"legacy entity_type ignored", "/entities/add", "entity_type=place", false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fa := &fakeApp{entities: []app.EntityResult{parent}}
			h := newActorHandler(t, fa, "")

			body := "display_name=Wrench"
			if tc.extra != "" {
				body += "&" + tc.extra
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, postForm(tc.path, body))

			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			assert.Equal(t, "Wrench", fa.lastCreateReq.DisplayName)
			assert.Equal(t, tc.wantLocked, fa.lastCreateReq.Locked, "Locked")
			assert.Equal(t, tc.wantDiscrete, fa.lastCreateReq.Discrete, "Discrete")
		})
	}
}

func TestAddItemForm_LockedDiscreteCheckboxes(t *testing.T) {
	h := newActorHandler(t, &fakeApp{}, "")
	req := httptest.NewRequest(http.MethodGet, "/entities/add", nil)
	req.Header.Set("Hx-Request", "true")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()

	assert.NotContains(t, body, "entity_type", "removed Type select must be gone")
	assert.Contains(t, body, `type="checkbox" name="locked"`)
	assert.Contains(t, body, `type="checkbox" name="discrete"`)
	assert.Contains(t, body, `for="locked"`)
	assert.Contains(t, body, `for="discrete"`)
	assert.Contains(t, body, "prevent direct reparenting")
	assert.Contains(t, body, "no children allowed")
	assert.NotContains(t, body, "checked", "checkboxes must default unchecked")
}

// #288: the request carries the picked parent's ID; no path round-trip.
func TestAddItem_PassesParentIDThrough(t *testing.T) {
	fa := &fakeApp{} // no entities: the handler must not pre-fetch the parent
	h := newActorHandler(t, fa, "")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, postForm("/entities/bin2/add", "display_name=Nail"))

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "bin2", fa.lastCreateReq.ParentID)
}

func TestAddItem_RootHasNoParentID(t *testing.T) {
	fa := &fakeApp{}
	h := newActorHandler(t, fa, "")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, postForm("/entities/add", "display_name=Nail"))

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Empty(t, fa.lastCreateReq.ParentID)
}

// Review Focus #3: app parent errors are client errors, not 500s.
func TestAddItem_ParentErrors(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"removed or unknown parent", fmt.Errorf("get entity %q: %w", "gone", app.ErrNotFound), http.StatusNotFound},
		{
			"discrete parent",
			fmt.Errorf("%q is discrete: %w", "Box", app.ErrInvalidDestination),
			http.StatusUnprocessableEntity,
		},
		{"other", errTest, http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fa := &fakeApp{err: tc.err}
			h := newActorHandler(t, fa, "")

			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, postForm("/entities/gone/add", "display_name=Nail"))

			assert.Equal(t, tc.want, rec.Code)
		})
	}
}

func createdTrigger(t *testing.T, rec *httptest.ResponseRecorder) map[string]string {
	t.Helper()
	var trigger map[string]map[string]string
	require.NoError(t, json.Unmarshal([]byte(rec.Header().Get("Hx-Trigger")), &trigger))
	ev, ok := trigger["entity-created"]
	require.True(t, ok, "entity-created missing")
	return ev
}

// Review Focus #1: the response must not depend on the parent's tree node being loaded.
func TestAddItem_RespondsWithParentDetailAndTrigger(t *testing.T) {
	fa := &fakeApp{
		entities: []app.EntityResult{
			{EntityID: "g", DisplayName: "Garage", FullPathDisplay: "Garage", Status: inventory.EntityStatusOk},
		},
		children:     map[string][]app.EntityResult{"g": {{EntityID: "b1", DisplayName: "Bench", ParentID: "g"}}},
		createResult: app.EntityResult{EntityID: "b1", DisplayName: "Bench", ParentID: "g"},
	}
	h := newActorHandler(t, fa, "")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, postForm("/entities/g/add", "display_name=Bench"))

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	body := rec.Body.String()
	assert.Equal(t, "/entities/g", rec.Header().Get("Hx-Push-Url"))
	assert.Equal(t, map[string]string{"entityId": "b1", "parentId": "g"}, createdTrigger(t, rec))
	assert.Contains(t, body, `<section id="entity-detail"`)
	assert.Contains(t, body, `<a class="child-row" href="/entities/b1"`)
	assert.NotContains(t, body, `role="treeitem"`, "tree insertion is the trigger's job now")
}

func TestAddItem_RootRespondsWithHome(t *testing.T) {
	fa := &fakeApp{
		entities: []app.EntityResult{
			{EntityID: "s", DisplayName: "Shed", FullPathDisplay: "Shed", Status: inventory.EntityStatusOk},
		},
		createResult: app.EntityResult{EntityID: "s", DisplayName: "Shed"},
	}
	h := newActorHandler(t, fa, "")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, postForm("/entities/add", "display_name=Shed"))

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "/", rec.Header().Get("Hx-Push-Url"))
	assert.Equal(t, map[string]string{"entityId": "s", "parentId": ""}, createdTrigger(t, rec))
	assert.Contains(t, rec.Body.String(), `<section id="home-detail"`)
}

func TestAddItem_ParentGoneAfterCreateShowsHome(t *testing.T) {
	fa := &fakeApp{createResult: app.EntityResult{EntityID: "n1", DisplayName: "Nail", ParentID: "bin2"}}
	h := newActorHandler(t, fa, "")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, postForm("/entities/bin2/add", "display_name=Nail"))

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "/", rec.Header().Get("Hx-Push-Url"))
	assert.Contains(t, rec.Body.String(), `<section id="home-detail"`)
}

func TestAddItemForm_TargetsDetailPanel(t *testing.T) {
	h := newActorHandler(t, &fakeApp{}, "")
	req := httptest.NewRequest(http.MethodGet, "/entities/g/add", nil)
	req.Header.Set("Hx-Request", "true")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `hx-target="#detail-panel"`)
	assert.NotContains(t, rec.Body.String(), "#tree-children-")
}
