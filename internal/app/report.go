package app

import (
	"cmp"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

// ReportPreset is one named Report (CONTEXT.md "Report"): a starting filter.
type ReportPreset struct {
	Slug   string
	Title  string
	Filter EntityFilter
}

// ReportSlugByParent names the By parent preset, which has no fixed filter:
// it is any filter with only Under set.
const ReportSlugByParent = "by-parent"

const (
	reportSlugAll     = "all"
	reportSlugCustom  = "custom"
	reportTitleCustom = "Custom filter"
	reportLabelAll    = "Everything"
	// reportFileTime is the UTC timestamp layout in export filenames.
	reportFileTime = "20060102T150405Z"
)

// ReportPresets returns the reports in display order.
func ReportPresets() []ReportPreset {
	return []ReportPreset{
		{Slug: "missing", Title: "Missing", Filter: EntityFilter{Status: inventory.EntityStatusMissing.String()}},
		{Slug: "loaned", Title: "Loaned", Filter: EntityFilter{Status: inventory.EntityStatusLoaned.String()}},
		{Slug: "borrowed", Title: "Borrowed", Filter: EntityFilter{Status: inventory.EntityStatusBorrowed.String()}},
		{Slug: "displaced", Title: "Displaced", Filter: EntityFilter{Displaced: true}},
		{Slug: ReportSlugByParent, Title: "By parent"},
		{Slug: reportSlugAll, Title: "All"},
	}
}

// MatchPreset returns the preset whose filter f equals exactly; false means f
// is a custom filter.
func MatchPreset(f EntityFilter) (ReportPreset, bool) {
	for _, p := range ReportPresets() {
		if p.Slug == ReportSlugByParent {
			if !f.Under.IsEmpty() && f == (EntityFilter{Under: f.Under}) {
				return p, true
			}
			continue
		}
		if f == p.Filter {
			return p, true
		}
	}
	return ReportPreset{}, false
}

// PresetCount is a preset and how many entities it matches now. By parent
// needs a parent to mean anything, so it has no count.
type PresetCount struct {
	ReportPreset

	Count    int
	HasCount bool
}

// CountPresets counts each preset's matches in all.
func CountPresets(all []EntityResult) []PresetCount {
	presets := ReportPresets()
	out := make([]PresetCount, len(presets))
	for i, p := range presets {
		out[i] = PresetCount{ReportPreset: p}
		if p.Slug == ReportSlugByParent {
			continue
		}
		out[i].HasCount = true
		for _, e := range all {
			if reportable(p.Filter, e) {
				out[i].Count++
			}
		}
	}
	return out
}

// reportable reports whether e belongs in a report filtered by f: removed
// entities never do.
func reportable(f EntityFilter, e EntityResult) bool {
	return e.Status != inventory.EntityStatusRemoved && f.Matches(e)
}

// Report is one run of the entity filter, ready to render or export.
type Report struct {
	Slug        string // preset slug, or "custom"
	Title       string // preset title, or "Custom filter"
	Filter      EntityFilter
	Database    string
	GeneratedAt time.Time      // UTC
	Entities    []EntityResult // matches, sorted by path (case-insensitive)
}

// BuildReport filters all by f, drops removed entities, and sorts by path.
func BuildReport(all []EntityResult, f EntityFilter, database string, now time.Time) Report {
	var matched []EntityResult
	for _, e := range all {
		if reportable(f, e) {
			matched = append(matched, e)
		}
	}
	slices.SortFunc(matched, func(a, b EntityResult) int {
		return cmp.Or(
			strings.Compare(strings.ToLower(a.FullPathDisplay), strings.ToLower(b.FullPathDisplay)),
			strings.Compare(a.FullPathDisplay, b.FullPathDisplay),
			strings.Compare(a.EntityID, b.EntityID),
		)
	})

	r := Report{
		Slug:        reportSlugCustom,
		Title:       reportTitleCustom,
		Filter:      f,
		Database:    database,
		GeneratedAt: now.UTC(),
		Entities:    matched,
	}
	if p, ok := MatchPreset(f); ok {
		r.Slug, r.Title = p.Slug, p.Title
	}
	return r
}

// Count is the number of matched entities.
func (r Report) Count() int { return len(r.Entities) }

// FilterLabel describes r's filter for its header, e.g.
// "Loaned · under Garage:Toolbox".
func (r Report) FilterLabel() string {
	var parts []string
	if s := r.Filter.Status; s != "" {
		parts = append(parts, strings.ToUpper(s[:1])+s[1:])
	}
	if r.Filter.Displaced {
		parts = append(parts, "Displaced")
	}
	if !r.Filter.Under.IsEmpty() {
		parts = append(parts, "under "+r.Filter.Under.String())
	}
	if !r.Filter.HomeUnder.IsEmpty() {
		parts = append(parts, "home under "+r.Filter.HomeUnder.String())
	}
	if len(parts) == 0 {
		return reportLabelAll
	}
	return strings.Join(parts, " · ")
}

var slugUnsafe = regexp.MustCompile(`[^a-z0-9]+`)

// maxSlugLen caps one path's slug so a filename with two of them stays well
// under the common 255-byte filename limit.
const maxSlugLen = 60

// slugify reduces s to at most maxSlugLen bytes of lowercase ASCII words
// joined by "-"; it may be "".
func slugify(s string) string {
	slug := strings.Trim(slugUnsafe.ReplaceAllString(strings.ToLower(s), "-"), "-")
	if len(slug) > maxSlugLen {
		slug = strings.TrimRight(slug[:maxSlugLen], "-")
	}
	return slug
}

// Filename is the export filename for r with extension ext ("csv", "json"):
// wherehouse-<filter parts>-<UTC timestamp>.<ext>, "all" when unfiltered.
// It is always plain ASCII.
func (r Report) Filename(ext string) string {
	parts := []string{"wherehouse"}
	if r.Filter.Status != "" {
		parts = append(parts, r.Filter.Status)
	}
	if r.Filter.Displaced {
		parts = append(parts, "displaced")
	}
	parts = appendPathParts(parts, "under", r.Filter.Under.String())
	parts = appendPathParts(parts, "home-under", r.Filter.HomeUnder.String())
	if len(parts) == 1 {
		parts = append(parts, reportSlugAll)
	}
	parts = append(parts, r.GeneratedAt.UTC().Format(reportFileTime))
	return strings.Join(parts, "-") + "." + ext
}

// appendPathParts appends key and path's slug (when non-empty) for a set path.
func appendPathParts(parts []string, key, path string) []string {
	if path == "" {
		return parts
	}
	parts = append(parts, key)
	if s := slugify(path); s != "" {
		parts = append(parts, s)
	}
	return parts
}
