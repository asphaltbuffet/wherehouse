package web

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"time"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/entitypath"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

const (
	formatCSV  = "csv"
	formatJSON = "json"
	// displacedOn is the only accepted value of ?displaced= (the checkbox's value).
	displacedOn = "1"
)

var errBadReportParam = errors.New("bad report parameter")

// reportParams is the report filter as it travels in the URL: entity IDs,
// not paths, so a bookmark survives a rename.
type reportParams struct {
	Status      string
	UnderID     string
	HomeUnderID string
	Displaced   bool
}

func parseReportParams(q url.Values) (reportParams, error) {
	p := reportParams{
		UnderID:     q.Get("under"),
		HomeUnderID: q.Get("home_under"),
	}
	if len(p.UnderID) > maxQueryLen || len(p.HomeUnderID) > maxQueryLen {
		return reportParams{}, fmt.Errorf("%w: entity id too long", errBadReportParam)
	}
	switch d := q.Get("displaced"); d {
	case "":
	case displacedOn:
		p.Displaced = true
	default:
		return reportParams{}, fmt.Errorf("%w: displaced %q (use %s)", errBadReportParam, d, displacedOn)
	}
	if s := q.Get("status"); s != "" {
		st, err := inventory.ParseEntityStatus(s)
		if err != nil || st == inventory.EntityStatusRemoved {
			return reportParams{}, fmt.Errorf("%w: status %q", errBadReportParam, s)
		}
		p.Status = st.String()
	}
	return p, nil
}

// href is the reports URL for p, plus format when non-empty.
func (p reportParams) href(format string) string {
	v := url.Values{}
	if p.Status != "" {
		v.Set("status", p.Status)
	}
	if p.UnderID != "" {
		v.Set("under", p.UnderID)
	}
	if p.Displaced {
		v.Set("displaced", displacedOn)
	}
	if p.HomeUnderID != "" {
		v.Set("home_under", p.HomeUnderID)
	}
	if format != "" {
		v.Set("format", format)
	}
	if len(v) == 0 {
		return "/reports"
	}
	return "/reports?" + v.Encode()
}

// reportPath resolves an entity ID to its path as a filter criterion; ""
// resolves to the empty (match-all) path.
func (s *Server) reportPath(ctx context.Context, entityID string) (entitypath.Path, error) {
	if entityID == "" {
		return "", nil
	}
	e, err := s.cfg.App.GetEntityByID(ctx, entityID)
	if err != nil {
		return "", fmt.Errorf("get entity %q: %w", entityID, err)
	}
	p, err := entitypath.Parse(e.FullPathDisplay)
	if err != nil {
		return "", fmt.Errorf("parse path %q: %w", e.FullPathDisplay, err)
	}
	return p, nil
}

func (s *Server) reportFilter(ctx context.Context, p reportParams) (app.EntityFilter, error) {
	under, err := s.reportPath(ctx, p.UnderID)
	if err != nil {
		return app.EntityFilter{}, err
	}
	homeUnder, err := s.reportPath(ctx, p.HomeUnderID)
	if err != nil {
		return app.EntityFilter{}, err
	}
	return app.EntityFilter{Status: p.Status, Under: under, HomeUnder: homeUnder, Displaced: p.Displaced}, nil
}

// handleReports serves the Reports page, or its CSV/JSON export when
// ?format= is set (CONTEXT.md "Report").
func (s *Server) handleReports(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	params, err := parseReportParams(q)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	format := q.Get("format")
	if format != "" && format != formatCSV && format != formatJSON {
		http.Error(w, fmt.Sprintf("%s: format %q", errBadReportParam, format), http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	filter, err := s.reportFilter(ctx, params)
	if errors.Is(err, app.ErrNotFound) {
		http.Error(w, "entity not found", http.StatusNotFound)
		return
	}
	if err != nil {
		s.serverError(w, "resolve report filter", err)
		return
	}
	all, err := s.cfg.App.ListEntities(ctx)
	if err != nil {
		s.serverError(w, "list entities", err)
		return
	}
	name, err := s.cfg.App.GetWherehouseName(ctx)
	if err != nil {
		s.serverError(w, "get wherehouse name", err)
		return
	}
	report := app.BuildReport(all, filter, name, time.Now())

	switch format {
	case formatCSV:
		s.writeReportCSV(w, report)
	case formatJSON:
		s.writeReportJSON(w, report)
	default:
		s.renderReportPage(w, r, params, report, all)
	}
}

// writeDownload sends body as an attachment named filename. The body is
// built before anything is written, so a failure is still a clean 500.
func (s *Server) writeDownload(w http.ResponseWriter, contentType, filename string, body []byte) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filename}))
	if _, err := w.Write(body); err != nil {
		s.cfg.Logger.Error("write download", "error", err, "filename", filename)
	}
}

func (s *Server) writeReportCSV(w http.ResponseWriter, report app.Report) {
	var buf bytes.Buffer
	if err := app.WriteReportCSV(&buf, report); err != nil {
		s.serverError(w, "write report csv", err)
		return
	}
	s.writeDownload(w, "text/csv; charset=utf-8", report.Filename(formatCSV), buf.Bytes())
}

func (s *Server) writeReportJSON(w http.ResponseWriter, report app.Report) {
	body, err := json.MarshalIndent(app.ToReportOutput(report), "", "  ")
	if err != nil {
		s.serverError(w, "marshal report json", err)
		return
	}
	s.writeDownload(w, "application/json", report.Filename(formatJSON), body)
}

// reportPresetView is one preset link; By parent has no Href and renders as
// a label that focuses the Under picker.
type reportPresetView struct {
	Title    string
	Href     string
	Count    int
	HasCount bool
	Current  bool
}

type reportFormView struct {
	Statuses      []string
	Status        string
	UnderID       string
	UnderPath     string
	HomeUnderID   string
	HomeUnderPath string
	Displaced     bool
}

type reportsData struct {
	Shell     pageShell
	Report    app.Report
	Generated string // RFC3339 UTC
	Presets   []reportPresetView
	Form      reportFormView
	CSVHref   string
	JSONHref  string
}

func (s *Server) renderReportPage(
	w http.ResponseWriter, r *http.Request, params reportParams, report app.Report, all []app.EntityResult,
) {
	shell, err := s.newPageShell(r.Context(), r, "Wherehouse — Reports")
	if err != nil {
		s.serverError(w, "build page shell", err)
		return
	}

	counts := app.CountPresets(all)
	presets := make([]reportPresetView, len(counts))
	for i, c := range counts {
		presets[i] = reportPresetView{
			Title:    c.Title,
			Count:    c.Count,
			HasCount: c.HasCount,
			Current:  c.Slug == report.Slug,
		}
		if c.Slug != app.ReportSlugByParent {
			presets[i].Href = reportParams{Status: c.Filter.Status, Displaced: c.Filter.Displaced}.href("")
		}
	}

	s.renderHTML(w, "reports", reportsData{
		Shell:     shell,
		Report:    report,
		Generated: report.GeneratedAt.Format(time.RFC3339),
		Presets:   presets,
		Form: reportFormView{
			Statuses: []string{
				inventory.EntityStatusOk.String(),
				inventory.EntityStatusMissing.String(),
				inventory.EntityStatusLoaned.String(),
				inventory.EntityStatusBorrowed.String(),
			},
			Status:        params.Status,
			UnderID:       params.UnderID,
			UnderPath:     report.Filter.Under.String(),
			HomeUnderID:   params.HomeUnderID,
			HomeUnderPath: report.Filter.HomeUnder.String(),
			Displaced:     params.Displaced,
		},
		CSVHref:  params.href(formatCSV),
		JSONHref: params.href(formatJSON),
	})
}
