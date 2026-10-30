package app

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
)

var reportCSVHeader = []string{
	"path", "status", "status_context", "home", "locked", "discrete", "tags", "entity_id",
}

// reportTagSep joins tags inside one CSV cell.
const reportTagSep = ";"

// formulaLeads are the leading characters a spreadsheet treats as a formula.
const formulaLeads = "=+-@\t\r"

// WriteReportCSV writes r as CSV: a header row, then one row per entity.
// Every cell is neutralized against spreadsheet formula injection.
func WriteReportCSV(w io.Writer, r Report) error {
	cw := csv.NewWriter(w)
	if err := cw.Write(reportCSVHeader); err != nil {
		return fmt.Errorf("write csv header: %w", err)
	}
	for _, row := range ToReportRows(r) {
		rec := []string{
			row.Path,
			row.Status.String(),
			row.StatusContext,
			row.Home,
			strconv.FormatBool(row.Locked),
			strconv.FormatBool(row.Discrete),
			strings.Join(row.Tags, reportTagSep),
			row.EntityID,
		}
		for i := range rec {
			rec[i] = csvSafe(rec[i])
		}
		if err := cw.Write(rec); err != nil {
			return fmt.Errorf("write csv row %s: %w", row.EntityID, err)
		}
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		return fmt.Errorf("flush csv: %w", err)
	}
	return nil
}

// csvSafe prefixes a cell that a spreadsheet would evaluate as a formula with
// a quote, so it opens as text.
func csvSafe(s string) string {
	if s != "" && strings.ContainsRune(formulaLeads, rune(s[0])) {
		return "'" + s
	}
	return s
}
