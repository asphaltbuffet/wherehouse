//go:build browser

package web_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/mxschmitt/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/app"
)

// minTouchTarget mirrors the 2.75rem `min-height` in app.css's
// `pointer: coarse` block (16px root font size).
const minTouchTarget = 44

// minPointerTarget is WCAG 2.5.8's minimum target size, which a control
// next to another target (a chevron beside its row) must meet on any pointer.
const minPointerTarget = 24

// settleTimeout matches the default timeout of the auto-retrying expect
// assertions.
const settleTimeout = 5 * time.Second

// overflowOffendersJS lists the document, if it is wider than the viewport,
// and every scroll container that scrolls sideways. body hides overflow, so a
// too-wide child of #detail-panel scrolls the panel, not the document.
const overflowOffendersJS = `() => [document.documentElement, ...document.querySelectorAll('*')]
	.filter(el => el === document.documentElement
		? el.scrollWidth > window.innerWidth
		: ['auto', 'scroll'].includes(getComputedStyle(el).overflowX) && el.scrollWidth > el.clientWidth)
	.map(el => el.tagName.toLowerCase() + (el.id ? '#' + el.id : '') +
		' (scrollWidth ' + el.scrollWidth + ', clientWidth ' + el.clientWidth + ')')`

// interactiveSelector matches the controls a user can tap.
const interactiveSelector = "a[href], button, input:not([type=hidden]), select, [tabindex='0']"

func TestBrowserNoHorizontalOverflow(t *testing.T) {
	t.Parallel()
	forEachDevice(t, func(s *session) {
		deep := s.createDeepPath()

		s.visit("/")
		require.NoError(s.t, expect.Locator(s.detailHeading("Inventory")).ToBeVisible())
		s.requireNoHorizontalOverflow()

		s.visit("/entities/" + deep.EntityID)
		require.NoError(s.t, expect.Locator(s.detailHeading(deep.DisplayName)).ToBeVisible())
		s.requireNoHorizontalOverflow()
	})
}

func TestBrowserTouchTargetSize(t *testing.T) {
	t.Parallel()
	// `pointer: coarse` sizing applies only on touch screens.
	forDevices(t, touchDevices(), sessionConfig{}, func(s *session) {
		deep := s.createDeepPath()
		s.createEntity("Spare Fuse", deep.EntityID) // a child row to tap

		s.visit("/entities/" + deep.EntityID)
		require.NoError(s.t, expect.Locator(s.detailHeading(deep.DisplayName)).ToBeVisible())

		controls, err := s.page.GetByRole(*playwright.AriaRoleMain).Locator(interactiveSelector).All()
		require.NoError(s.t, err)
		require.NotEmpty(s.t, controls)

		var small []string
		for _, c := range controls {
			visible, visErr := c.IsVisible()
			require.NoError(s.t, visErr)
			if !visible {
				continue
			}
			box, boxErr := c.BoundingBox()
			require.NoError(s.t, boxErr)
			if box.Height < minTouchTarget {
				small = append(small, fmt.Sprintf("%q (%.1fpx)", s.controlName(c), box.Height))
			}
		}
		assert.Empty(s.t, small, "controls shorter than the %dpx touch target", minTouchTarget)
	})
}

// The chevron is the only mouse target that expands a row, and a near-miss
// selects the row instead (#364), so it must meet the minimum target size.
func TestBrowserChevronTargetSize(t *testing.T) {
	t.Parallel()
	forEachDevice(t, func(s *session) {
		garage := s.createEntity("Garage", "")
		s.createEntity("Toolbox", garage.EntityID)

		s.visit("/")
		s.openTree()

		chevron := s.treeRow("Garage").Locator(".chevron")
		require.NoError(s.t, expect.Locator(chevron).ToBeVisible())
		box, err := chevron.BoundingBox()
		require.NoError(s.t, err)
		assert.GreaterOrEqual(s.t, box.Width, float64(minPointerTarget), "chevron width")
		assert.GreaterOrEqual(s.t, box.Height, float64(minPointerTarget), "chevron height")
	})
}

// createDeepPath seeds a chain of long-named entities, deep enough that the
// breadcrumbs wrap on every device, and returns the deepest.
func (s *session) createDeepPath() app.EntityResult {
	s.t.Helper()
	parentID := ""
	var e app.EntityResult
	for _, name := range []string{
		"Basement Storage Room",
		"North Wall Shelving Unit",
		"Top Shelf Left Side",
		"Clear Plastic Bin Number Three",
		"Holiday Decorations Box",
		"Spare Parts For String Lights",
	} {
		e = s.createEntity(name, parentID)
		parentID = e.EntityID
	}
	return e
}

// requireNoHorizontalOverflow waits for nothing on the page to scroll
// sideways, failing with the offenders if it does not settle.
func (s *session) requireNoHorizontalOverflow() {
	s.t.Helper()
	settled := "() => (" + overflowOffendersJS + ")().length === 0"
	opts := playwright.PageWaitForFunctionOptions{Timeout: new(float64(settleTimeout.Milliseconds()))}
	if _, err := s.page.WaitForFunction(settled, nil, opts); err == nil {
		return
	}
	offenders, err := s.page.Evaluate(overflowOffendersJS)
	require.NoError(s.t, err)
	s.t.Fatalf("horizontal overflow: %v", offenders)
}

// controlName describes c for a failure message: its text, else its label.
func (s *session) controlName(c playwright.Locator) string {
	s.t.Helper()
	text, err := c.InnerText()
	require.NoError(s.t, err)
	if text = strings.TrimSpace(text); text != "" {
		return text
	}
	label, err := c.GetAttribute("aria-label")
	require.NoError(s.t, err)
	return label
}
