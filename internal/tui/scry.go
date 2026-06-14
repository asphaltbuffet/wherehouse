package tui

import (
	"context"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/styles"
)

// scryUIOverhead is the number of terminal rows used by title, input, and help bar.
const scryUIOverhead = 4

type scryModel struct {
	input   textinput.Model
	results list.Model
	appRef  App
	st      *styles.Styles
}

// scryItem wraps app.FindResult for the bubbles/list.
type scryItem struct {
	result app.FindResult
}

func (s scryItem) FilterValue() string { return s.result.Entity.DisplayName }
func (s scryItem) Title() string       { return s.result.Entity.FullPathDisplay }
func (s scryItem) Description() string {
	return fmt.Sprintf("[%s] dist:%d", s.result.Entity.Status, s.result.Distance)
}

func newScryModel(a App, st *styles.Styles) scryModel {
	inp := textinput.New()
	inp.Placeholder = "search inventory..."
	inp.Focus()

	d := list.NewDefaultDelegate()
	l := list.New(nil, d, 0, 0)
	l.SetShowTitle(false)
	l.SetShowFilter(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.DisableQuitKeybindings()

	return scryModel{
		input:   inp,
		results: l,
		appRef:  a,
		st:      st,
	}
}

func (s scryModel) searchCmd() tea.Cmd {
	query := s.input.Value()
	a := s.appRef
	return func() tea.Msg {
		results, err := a.FindEntities(context.Background(), app.FindEntitiesRequest{Query: query})
		return scryResultsMsg{items: results, err: err}
	}
}

func (s scryModel) Update(msg tea.Msg) (scryModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case keyEsc:
			return s, func() tea.Msg { return scryCancelledMsg{} }

		case "enter":
			sel, ok := s.results.SelectedItem().(scryItem)
			if !ok {
				return s, nil
			}
			entity := sel.result.Entity
			a := s.appRef
			return s, s.navigateCmd(entity, a)
		}

		var cmd tea.Cmd
		s.input, cmd = s.input.Update(msg)
		return s, tea.Batch(cmd, s.searchCmd())

	case scryResultsMsg:
		if msg.err != nil {
			return s, nil
		}
		items := make([]list.Item, len(msg.items))
		for i, r := range msg.items {
			items[i] = scryItem{result: r}
		}
		s.results.SetItems(items)
		return s, nil

	case tea.WindowSizeMsg:
		s.results.SetSize(msg.Width, msg.Height-scryUIOverhead)
		return s, nil
	}

	var cmd tea.Cmd
	s.results, cmd = s.results.Update(msg)
	return s, cmd
}

type scryCancelledMsg struct{}

// navigateCmd loads root entities and returns a scryNavigatedMsg targeting the entity.
// EntityResult does not carry parentID, so we always reload from root and let the
// Update handler position the cursor by ID. Deep navigation is a future improvement
// once EntityResult exposes parentID.
func (s scryModel) navigateCmd(entity app.EntityResult, _ App) tea.Cmd {
	return func() tea.Msg {
		return scryNavigatedMsg{
			targetEntityID: entity.EntityID,
		}
	}
}

func (s scryModel) View(width, height int) string {
	title := s.st.TUIDetailLabel().Render("scry")
	inp := s.input.View()
	results := s.results.View()
	help := s.st.Muted().Render("[enter] navigate  [esc] cancel")
	return lipgloss.NewStyle().Width(width).Height(height).Render(
		strings.Join([]string{title, inp, results, help}, "\n"),
	)
}
