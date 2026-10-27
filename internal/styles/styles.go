package styles

import (
	"os"
	"strings"

	"charm.land/lipgloss/v2"
)

// Styles holds the application's pre-built lipgloss styles. Fields are private;
// use the public accessor methods to read them. Duplicate style definitions
// share a single backing field, cutting storage from 93 fields to 39.
type Styles struct {
	fgTextDim    lipgloss.Style
	fgTextMid    lipgloss.Style
	fgTextBright lipgloss.Style
	fgAccent     lipgloss.Style
	fgSecondary  lipgloss.Style
	fgSuccess    lipgloss.Style
	fgWarning    lipgloss.Style
	fgDanger     lipgloss.Style
	fgMuted      lipgloss.Style

	fgTextDimBold     lipgloss.Style
	fgTextDimItalic   lipgloss.Style
	fgAccentBold      lipgloss.Style
	fgAccentItalic    lipgloss.Style
	fgSecondaryBold   lipgloss.Style
	fgSecondaryItalic lipgloss.Style
	fgSuccessBold     lipgloss.Style
	fgSuccessItalic   lipgloss.Style
	fgDangerBold      lipgloss.Style
	fgWarningBold     lipgloss.Style

	bold lipgloss.Style
	base lipgloss.Style
}

// Colorblind-safe palette (Wong) with adaptive light/dark variants.
//
// Each color uses lipgloss.LightDark(Light, Dark) so the UI looks
// correct on both dark and light terminal backgrounds. The Light values
// are darkened/saturated versions of the Dark values to maintain contrast
// on white backgrounds.
//
// Chromatic roles:
//
//	Primary accent:   sky blue     Dark #56B4E9  Light #0072B2
//	Secondary accent: orange       Dark #E69F00  Light #D55E00
//	Success/positive: bluish green Dark #009E73  Light #007A5A
//	Warning:          yellow       Dark #F0E442  Light #B8860B
//	Error/danger:     vermillion   Dark #D55E00  Light #CC3311
//	Muted accent:     rose         Dark #CC79A7  Light #AA4499
//
// Neutral roles:
//
//	Text bright:      Dark #E5E7EB  Light #1F2937
//	Text mid:         Dark #9CA3AF  Light #4B5563
//	Text dim:         Dark #6B7280  Light #4B5563
//	Surface:          Dark #1F2937  Light #F3F4F6
//	Surface deep:     Dark #111827  Light #E5E7EB
//	On-accent text:   Dark #0F172A  Light #FFFFFF
var (
	// colors.
	skyblueLight = lipgloss.Color("#0072B2")
	skyblueDark  = lipgloss.Color("#56B4E9")

	// adaptive helpers.
	hasDarkBG = lipgloss.HasDarkBackground(os.Stdin, os.Stdout)
	adaptive  = lipgloss.LightDark(hasDarkBG)

	// adaptive colors.
	accent    = adaptive(skyblueLight, skyblueDark)
	secondary = adaptive(lipgloss.Color("#D55E00"), lipgloss.Color("#E69F00"))
	success   = adaptive(lipgloss.Color("#007A5A"), lipgloss.Color("#009E73"))
	warning   = adaptive(lipgloss.Color("#B8860B"), lipgloss.Color("#F0E442"))
	danger    = adaptive(lipgloss.Color("#CC3311"), lipgloss.Color("#D55E00"))
	muted     = adaptive(lipgloss.Color("#AA4499"), lipgloss.Color("#CC79A7"))

	textBright = adaptive(lipgloss.Color("#1F2937"), lipgloss.Color("#E5E7EB"))
	textMid    = adaptive(lipgloss.Color("#4B5563"), lipgloss.Color("#9CA3AF"))
	textDim    = adaptive(lipgloss.Color("#4B5563"), lipgloss.Color("#6B7280"))
)

// DefaultStyles returns a pointer to all application styles.
func DefaultStyles() *Styles {
	return &Styles{
		fgTextDim:    lipgloss.NewStyle().Foreground(textDim),
		fgTextMid:    lipgloss.NewStyle().Foreground(textMid),
		fgTextBright: lipgloss.NewStyle().Foreground(textBright),
		fgAccent:     lipgloss.NewStyle().Foreground(accent),
		fgSecondary:  lipgloss.NewStyle().Foreground(secondary),
		fgSuccess:    lipgloss.NewStyle().Foreground(success),
		fgWarning:    lipgloss.NewStyle().Foreground(warning),
		fgDanger:     lipgloss.NewStyle().Foreground(danger),
		fgMuted:      lipgloss.NewStyle().Foreground(muted),

		fgTextDimBold:     lipgloss.NewStyle().Foreground(textDim).Bold(true),
		fgTextDimItalic:   lipgloss.NewStyle().Foreground(textDim).Italic(true),
		fgAccentBold:      lipgloss.NewStyle().Foreground(accent).Bold(true),
		fgAccentItalic:    lipgloss.NewStyle().Foreground(accent).Italic(true),
		fgSecondaryBold:   lipgloss.NewStyle().Foreground(secondary).Bold(true),
		fgSecondaryItalic: lipgloss.NewStyle().Foreground(secondary).Italic(true),
		fgSuccessBold:     lipgloss.NewStyle().Foreground(success).Bold(true),
		fgSuccessItalic:   lipgloss.NewStyle().Foreground(success).Italic(true),
		fgDangerBold:      lipgloss.NewStyle().Foreground(danger).Bold(true),
		fgWarningBold:     lipgloss.NewStyle().Foreground(warning).Bold(true),

		bold: lipgloss.NewStyle().Bold(true),
		base: lipgloss.NewStyle(),
	}
}

// --- Foreground(textDim) ---

// TextDim is a fg style for dim text.
func (s *Styles) TextDim() lipgloss.Style { return s.fgTextDim }

// --- Foreground(textMid) ---

// --- Foreground(textBright) ---

// --- Foreground(accent) ---

// AccentText is a formatting style.
func (s *Styles) AccentText() lipgloss.Style { return s.fgAccent }

// KVKey is a formatting style for KV keys.
func (s *Styles) KVKey() lipgloss.Style { return s.fgAccent }

// --- Foreground(secondary) ---

// SecondaryText is a formatting style.
func (s *Styles) SecondaryText() lipgloss.Style { return s.fgSecondary }

// KVValue is a formatting style for KV values.
func (s *Styles) KVValue() lipgloss.Style { return s.fgSecondary }

// --- Foreground(success) ---

// SuccessText is a formatting style.
func (s *Styles) SuccessText() lipgloss.Style { return s.fgSuccess }

// MoveOk is a formatting style.
func (s *Styles) MoveOk() lipgloss.Style { return s.fgSuccess }

// --- Foreground(warning) ---

// WarningText is a formatting style.
func (s *Styles) WarningText() lipgloss.Style { return s.fgWarning }

// --- Foreground(danger) ---

// DangerText is a formatting style.
func (s *Styles) DangerText() lipgloss.Style { return s.fgDanger }

// MoveFail is a formatting style.
func (s *Styles) MoveFail() lipgloss.Style { return s.fgDanger }

// --- Foreground(muted) ---

// Muted is a formatting style.
func (s *Styles) Muted() lipgloss.Style { return s.fgMuted }

// --- Foreground(border) ---

// --- Foreground + bold ---

// DimBold is a formatting style.
func (s *Styles) DimBold() lipgloss.Style { return s.fgTextDimBold }

// --- Foreground + italic ---

// Null is a formatting style.
func (s *Styles) Null() lipgloss.Style { return s.fgTextDimItalic }

// ItalicDim is a formatting style.
func (s *Styles) ItalicDim() lipgloss.Style { return s.fgTextDimItalic }

// --- Foreground(accent) + bold ---

// AccentBold is a formatting style.
func (s *Styles) AccentBold() lipgloss.Style { return s.fgAccentBold }

// --- Foreground(accent) + italic ---

// AccentItalic is a formatting style.
func (s *Styles) AccentItalic() lipgloss.Style { return s.fgAccentItalic }

// --- Foreground(secondary) + bold ---

// SecondaryBold is a formatting style.
func (s *Styles) SecondaryBold() lipgloss.Style { return s.fgSecondaryBold }

// --- Foreground(secondary) + italic ---

// SecondaryItalic is a formatting style.
func (s *Styles) SecondaryItalic() lipgloss.Style { return s.fgSecondaryItalic }

// --- Foreground(success) + bold ---

// Info is a formatting style.
func (s *Styles) Info() lipgloss.Style { return s.fgSuccessBold }

// --- Foreground(success) + italic ---

// SuccessItalic is a formatting style.
func (s *Styles) SuccessItalic() lipgloss.Style { return s.fgSuccessItalic }

// --- Foreground(danger) + bold ---

// Error is a formatting style for errors.
func (s *Styles) Error() lipgloss.Style { return s.fgDangerBold }

// --- Complex / unique ---

// Bold is a bold formatting style.
func (s *Styles) Bold() lipgloss.Style { return s.bold }

// Base is a basic formatting style.
func (s *Styles) Base() lipgloss.Style { return s.base }

// --- Map-lookup methods ---

// ItemStyle is a style for item names.
func (s *Styles) ItemStyle(isTemp bool) lipgloss.Style {
	if isTemp {
		return s.fgAccentItalic
	}

	return s.fgAccent
}

// LocationStyle is a style based on location name.
func (s *Styles) LocationStyle(key string) lipgloss.Style {
	switch strings.ToLower(key) {
	case "missing":
		return s.fgDangerBold
	case "borrowed":
		return s.fgSecondary
	case "loaned":
		return s.fgWarningBold
	default:
		return s.fgAccentBold
	}
}

// EventStyle is a style based on event type.
func (s *Styles) EventStyle(key string) lipgloss.Style {
	switch strings.ToLower(key) {
	case "item.removed", "location.removed":
		return s.fgTextDimItalic
	case "item.moved", "location.reparented":
		return s.fgAccentBold
	case "item.created", "location.created":
		return s.fgMuted
	case "item.missing":
		return s.fgDangerBold
	case "item.renamed", "location.renamed":
		return s.fgMuted
	case "item.borrowed":
		return s.fgWarning
	case "item.loaned":
		return s.fgWarningBold
	case "item.found":
		return s.fgSuccessBold
	default:
		return s.base
	}
}
