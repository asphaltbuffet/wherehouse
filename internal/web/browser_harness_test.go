//go:build browser

package web_test

import (
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/mxschmitt/playwright-go"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/apptesting"
	"github.com/asphaltbuffet/wherehouse/internal/web"
)

// Browser tests drive the real UI in headless browsers across a device matrix
// (ADR 0038). Run them with `mise run test-browser`.

// artifactsEnv names the directory that receives a trace and screenshot for
// each failing device subtest. Unset disables tracing.
const artifactsEnv = "BROWSER_ARTIFACTS_DIR"

// narrowMaxWidth mirrors the `max-width: 48rem` breakpoint in app.css and
// tree.js (16px root font size): viewports this wide or narrower get the
// drawer layout.
const narrowMaxWidth = 768

// deviceNames is the emulated matrix; descriptors come from Playwright.
var deviceNames = []string{"Desktop Chrome", "Desktop Firefox", "iPad (gen 7)", "iPad Mini", "Pixel 7"}

type layout int

const (
	wide layout = iota
	narrow
)

func (l layout) String() string {
	if l == narrow {
		return "narrow"
	}
	return "wide"
}

type device struct {
	name   string
	desc   *playwright.DeviceDescriptor
	layout layout
}

// Shared across the package; each device subtest gets its own context.
var (
	browsers map[string]playwright.Browser // keyed by engine name
	devices  []device
	expect   = playwright.NewPlaywrightAssertions()
)

func TestMain(m *testing.M) {
	os.Exit(runBrowserTests(m))
}

func runBrowserTests(m *testing.M) int {
	pw, err := playwright.Run()
	if err != nil {
		fmt.Fprintln(os.Stderr, "start playwright:", err)
		return 1
	}
	defer func() { _ = pw.Stop() }()

	engines := map[string]playwright.BrowserType{
		"chromium": pw.Chromium,
		"firefox":  pw.Firefox,
		"webkit":   pw.WebKit,
	}
	browsers = map[string]playwright.Browser{}
	for _, name := range deviceNames {
		desc, known := pw.Devices[name]
		if !known {
			fmt.Fprintf(os.Stderr, "unknown device %q\n", name)
			return 1
		}
		l := wide
		if desc.Viewport.Width <= narrowMaxWidth {
			l = narrow
		}
		devices = append(devices, device{name: name, desc: desc, layout: l})

		engine := desc.DefaultBrowserType
		if _, launched := browsers[engine]; launched {
			continue
		}
		b, launchErr := engines[engine].Launch()
		if launchErr != nil {
			fmt.Fprintf(os.Stderr, "launch %s: %v\n", engine, launchErr)
			return 1
		}
		defer func() { _ = b.Close() }()
		browsers[engine] = b
	}

	return m.Run()
}

// session is one device subtest's isolated view of a fresh server and
// database. Its helpers fail s.t.
type session struct {
	t      *testing.T
	app    *app.App
	page   playwright.Page
	layout layout
	touch  bool // the device has a touch screen, so `pointer: coarse` applies
}

// sessionConfig adjusts a session's browser context; the zero value is the
// device's defaults.
type sessionConfig struct {
	colorScheme *playwright.ColorScheme // nil leaves prefers-color-scheme at light
	// actorHeader configures the server's identity header (ADR 0031) and
	// has the browser send it with value actor, as a trusted proxy would.
	actorHeader string
	actor       string
}

// forEachDevice runs fn as a parallel subtest per device, each against its own
// server and database seeded through s.app.
func forEachDevice(t *testing.T, fn func(s *session)) {
	t.Helper()
	forEachDeviceWith(t, sessionConfig{}, fn)
}

// forEachDeviceWith is forEachDevice with each session adjusted by cfg.
func forEachDeviceWith(t *testing.T, cfg sessionConfig, fn func(s *session)) {
	t.Helper()
	for _, d := range devices {
		t.Run(d.name, func(t *testing.T) {
			t.Parallel()
			fn(newSession(t, d, cfg))
		})
	}
}

func newSession(t *testing.T, d device, cfg sessionConfig) *session {
	t.Helper()
	a := apptesting.OpenApp(t)
	srv, err := web.New(web.Config{
		App:         a,
		Bind:        "127.0.0.1",
		Port:        0,
		Output:      io.Discard,
		ActorHeader: cfg.actorHeader,
	})
	require.NoError(t, err)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	var headers map[string]string
	if cfg.actorHeader != "" {
		headers = map[string]string{cfg.actorHeader: cfg.actor}
	}

	ctx, err := browsers[d.desc.DefaultBrowserType].NewContext(playwright.BrowserNewContextOptions{
		BaseURL:           new(ts.URL),
		UserAgent:         new(d.desc.UserAgent),
		Viewport:          d.desc.Viewport,
		Screen:            d.desc.Screen,
		DeviceScaleFactor: new(d.desc.DeviceScaleFactor),
		IsMobile:          new(d.desc.IsMobile),
		HasTouch:          new(d.desc.HasTouch),
		ColorScheme:       cfg.colorScheme,
		ExtraHttpHeaders:  headers,
	})
	require.NoError(t, err)
	page, err := ctx.NewPage()
	require.NoError(t, err)

	artifacts := os.Getenv(artifactsEnv)
	if artifacts != "" {
		require.NoError(t, ctx.Tracing().Start(playwright.TracingStartOptions{
			Screenshots: new(true),
			Snapshots:   new(true),
		}))
	}
	// Registered after ts.Close, so it runs first: the context closes while
	// the server is still up.
	t.Cleanup(func() {
		if artifacts != "" && t.Failed() {
			saveArtifacts(t, ctx, page, filepath.Join(artifacts, t.Name()))
		}
		_ = ctx.Close()
	})

	return &session{t: t, app: a, page: page, layout: d.layout, touch: d.desc.HasTouch}
}

// requireLayout skips the device subtest unless it has layout l.
func (s *session) requireLayout(l layout) {
	s.t.Helper()
	if s.layout != l {
		s.t.Skipf("%s layout only", l)
	}
}

func saveArtifacts(t *testing.T, ctx playwright.BrowserContext, page playwright.Page, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Logf("create artifacts dir: %v", err)
		return
	}
	if _, err := page.Screenshot(playwright.PageScreenshotOptions{
		Path:     new(filepath.Join(dir, "screenshot.png")),
		FullPage: new(true),
	}); err != nil {
		t.Logf("save screenshot: %v", err)
	}
	if err := ctx.Tracing().Stop(filepath.Join(dir, "trace.zip")); err != nil {
		t.Logf("save trace: %v", err)
	}
	t.Logf("browser artifacts: %s (view trace with `playwright show-trace`)", dir)
}

// visit loads path (relative to the session's server) and waits for the page.
func (s *session) visit(path string) {
	s.t.Helper()
	_, err := s.page.Goto(path)
	require.NoError(s.t, err)
}

// createEntity seeds an entity through the app layer; parentID "" is a root.
func (s *session) createEntity(name, parentID string) app.EntityResult {
	s.t.Helper()
	e, err := s.app.CreateEntity(s.t.Context(), app.CreateEntityRequest{
		DisplayName: name,
		ParentID:    parentID,
		ActorID:     "browser-test",
	})
	require.NoError(s.t, err)
	return e
}

// openTree reveals the inventory tree: on narrow layouts it opens the drawer;
// on wide layouts the tree is always visible.
func (s *session) openTree() {
	s.t.Helper()
	if s.layout != narrow {
		return
	}
	toggle := s.treeToggle()
	require.NoError(s.t, toggle.Click())
	require.NoError(s.t, expect.Locator(toggle).ToHaveAttribute("aria-expanded", "true"))
}

// closeTree undoes openTree. The open drawer's backdrop covers the toggle,
// so it closes with Escape.
func (s *session) closeTree() {
	s.t.Helper()
	if s.layout != narrow {
		return
	}
	require.NoError(s.t, s.page.Keyboard().Press("Escape"))
	require.NoError(s.t, expect.Locator(s.treeToggle()).ToHaveAttribute("aria-expanded", "false"))
}

func (s *session) treeToggle() playwright.Locator {
	return s.page.GetByRole(*playwright.AriaRoleButton, playwright.PageGetByRoleOptions{
		Name:  "Inventory",
		Exact: new(true),
	})
}

// treePanel is the inventory tree's navigation landmark.
func (s *session) treePanel() playwright.Locator {
	return s.page.GetByRole(*playwright.AriaRoleNavigation, playwright.PageGetByRoleOptions{
		Name: "Inventory tree",
	})
}

func (s *session) treeItem(name string) playwright.Locator {
	return s.page.GetByRole(*playwright.AriaRoleTreeitem, playwright.PageGetByRoleOptions{
		Name:  name,
		Exact: new(true),
	})
}

// treeRow is the clickable row of a tree item, excluding its children.
func (s *session) treeRow(name string) playwright.Locator {
	return s.treeItem(name).Locator(":scope > .tree-item")
}

// expandTreeItem expands a tree item via its chevron, which (unlike the row)
// expands without loading the item's detail, and waits for its children.
func (s *session) expandTreeItem(name string) {
	s.t.Helper()
	require.NoError(s.t, s.treeRow(name).Locator(".chevron").Click())
	require.NoError(s.t, expect.Locator(s.treeItem(name)).ToHaveAttribute("aria-expanded", "true"))
}

// button is the button named name in the main content area.
func (s *session) button(name string) playwright.Locator {
	main := s.page.GetByRole(*playwright.AriaRoleMain)
	return main.GetByRole(*playwright.AriaRoleButton, playwright.LocatorGetByRoleOptions{
		Name:  name,
		Exact: new(true),
	})
}

// childLink is the row for name in the shown entity's (or home's) contents.
func (s *session) childLink(name string) playwright.Locator {
	contents := s.page.GetByRole(*playwright.AriaRoleRegion, playwright.PageGetByRoleOptions{Name: "Contents"})
	return contents.GetByRole(*playwright.AriaRoleLink, playwright.LocatorGetByRoleOptions{
		Name:  name,
		Exact: new(true),
	})
}

// requireChild returns parentID's live child named name ("" for a root),
// failing if the database has none.
func (s *session) requireChild(parentID, name string) app.EntityResult {
	s.t.Helper()
	live, err := s.app.ListEntities(s.t.Context())
	require.NoError(s.t, err)
	for _, e := range live {
		if e.ParentID == parentID && e.DisplayName == name {
			return e
		}
	}
	require.Failf(s.t, "child not in database", "no %q under parent %q", name, parentID)
	return app.EntityResult{}
}

// stored returns the database's current state of entityID, removed or not.
func (s *session) stored(entityID string) app.EntityResult {
	s.t.Helper()
	e, err := s.app.GetEntityByIDAnyStatus(s.t.Context(), entityID)
	require.NoError(s.t, err)
	return e
}

// mainText is the exact text text in the main content area.
func (s *session) mainText(text string) playwright.Locator {
	return s.page.GetByRole(*playwright.AriaRoleMain).GetByText(text, playwright.LocatorGetByTextOptions{
		Exact: new(true),
	})
}

// detailHeading is the entity name heading in the main content area.
func (s *session) detailHeading(name string) playwright.Locator {
	main := s.page.GetByRole(*playwright.AriaRoleMain)
	return main.GetByRole(*playwright.AriaRoleHeading, playwright.LocatorGetByRoleOptions{
		Name:  name,
		Level: new(2),
	})
}
