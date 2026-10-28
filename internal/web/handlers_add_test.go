package web_test

import (
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
		{"discrete parent", fmt.Errorf("%q is discrete: %w", "Box", app.ErrInvalidDestination), http.StatusUnprocessableEntity},
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
