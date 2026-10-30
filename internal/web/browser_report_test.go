//go:build browser

package web_test

import (
	"regexp"
	"testing"

	"github.com/mxschmitt/playwright-go"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/app"
)

func (s *session) markLoaned(entityID, to string) {
	s.t.Helper()
	_, err := s.app.MarkLoaned(s.t.Context(), []app.ChangeStatusRequest{
		{EntityID: entityID, StatusContext: to, ActorID: "browser-test"},
	})
	require.NoError(s.t, err)
}

func (s *session) reportRow(path string) playwright.Locator {
	return s.page.Locator(".report-table tbody tr").Filter(playwright.LocatorFilterOptions{HasText: path})
}

func TestBrowserReportsPresets(t *testing.T) {
	t.Parallel()
	forEachDevice(t, func(s *session) {
		garage := s.createEntity("Garage", "")
		drill := s.createEntity("Drill", garage.EntityID)
		s.createEntity("Ladder", garage.EntityID)
		s.markLoaned(drill.EntityID, "Sam")

		s.visit("/")
		s.click(s.page.GetByRole(*playwright.AriaRoleLink, playwright.PageGetByRoleOptions{
			Name:  "Reports",
			Exact: new(true),
		}))
		require.NoError(s.t, expect.Locator(s.reportRow("Garage:Ladder")).ToBeVisible())

		s.click(s.page.Locator(`.report-preset[href="/reports?status=loaned"]`))
		require.NoError(s.t, expect.Locator(s.reportRow("Garage:Drill")).ToContainText("Sam"))
		require.NoError(s.t, expect.Locator(s.reportRow("Garage:Ladder")).ToHaveCount(0))

		wantDisplay := "table-row"
		if s.layout == narrow {
			wantDisplay = "block"
		}
		require.NoError(s.t, expect.Locator(s.reportRow("Garage:Drill")).ToHaveCSS("display", wantDisplay))
	})
}

func TestBrowserReportsPickers(t *testing.T) {
	t.Parallel()
	forEachDevice(t, func(s *session) {
		garage := s.createEntity("Garage", "")
		kitchen := s.createEntity("Kitchen", "")
		s.createEntity("Drill", garage.EntityID)

		s.visit("/reports")
		s.fill(s.page.Locator("#report_under_q"), "Gar")
		s.click(s.page.Locator(`#report_under_options [data-parent-id="` + garage.EntityID + `"]`))
		s.fill(s.page.Locator("#report_home_under_q"), "Kit")
		s.click(s.page.Locator(`#report_home_under_options [data-parent-id="` + kitchen.EntityID + `"]`))
		s.click(s.button("Apply"))

		require.NoError(s.t, expect.Page(s.page).ToHaveURL(regexp.MustCompile(
			`/reports\?status=&under=`+garage.EntityID+`&home_under=`+kitchen.EntityID+`$`)))
		require.NoError(s.t, expect.Locator(s.mainText("under Garage · home under Kitchen")).ToBeVisible())
	})
}

func TestBrowserReportsDetailLinkAndPrint(t *testing.T) {
	t.Parallel()
	forEachDevice(t, func(s *session) {
		garage := s.createEntity("Garage", "")
		s.createEntity("Drill", garage.EntityID)

		s.visit("/entities/" + garage.EntityID)
		s.click(s.page.GetByRole(*playwright.AriaRoleMain).GetByRole(*playwright.AriaRoleLink,
			playwright.LocatorGetByRoleOptions{Name: "Report", Exact: new(true)}))
		require.NoError(s.t, expect.Locator(s.page.Locator(".report-title")).ToHaveText("By parent"))
		require.NoError(s.t, expect.Locator(s.reportRow("Garage:Drill")).ToBeVisible())

		require.NoError(s.t, s.page.EmulateMedia(playwright.PageEmulateMediaOptions{Media: playwright.MediaPrint}))
		require.NoError(s.t, expect.Locator(s.page.Locator(".report-filter")).ToBeHidden())
		require.NoError(s.t, expect.Locator(s.page.Locator(".app-header")).ToBeHidden())
		require.NoError(s.t, expect.Locator(s.page.Locator(".report-meta")).ToBeVisible())
		require.NoError(s.t, expect.Locator(s.reportRow("Garage:Drill")).ToBeVisible())
	})
}

func TestBrowserReportsStatusLabelSitsOverItsSelect(t *testing.T) {
	t.Parallel()
	forEachDevice(t, func(s *session) {
		s.visit("/reports")
		label, err := s.page.Locator(`label[for="report_status"]`).BoundingBox()
		require.NoError(s.t, err)
		sel, err := s.page.Locator("#report_status").BoundingBox()
		require.NoError(s.t, err)
		require.InDelta(s.t, label.X, sel.X, 1, "label and select share a column")
		require.LessOrEqual(s.t, label.Y+label.Height, sel.Y+1, "label sits above its select")
	})
}
