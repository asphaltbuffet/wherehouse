package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
	"github.com/asphaltbuffet/wherehouse/internal/styles"
)

type confirmKind int

const (
	confirmLost   confirmKind = iota
	confirmReturn             // also handles borrowed→removed
	confirmFound
)

type confirmModel struct {
	kind   confirmKind
	entity app.EntityResult
	note   textinput.Model
	appRef App
	st     *styles.Styles
}

func newConfirmModel(kind confirmKind, entity app.EntityResult, a App, st *styles.Styles) confirmModel {
	note := textinput.New()
	note.Placeholder = notePlaceholder
	note.Focus()
	return confirmModel{
		kind:   kind,
		entity: entity,
		note:   note,
		appRef: a,
		st:     st,
	}
}

func (c confirmModel) targetStatusLabel() string {
	switch c.kind {
	case confirmLost:
		return "missing"
	case confirmReturn:
		return "returned"
	case confirmFound:
		return "ok (found)"
	}
	return ""
}

func (c confirmModel) Update(msg tea.Msg) (confirmModel, tea.Cmd) {
	kMsg, isKey := msg.(tea.KeyPressMsg)
	if !isKey {
		return c, nil
	}
	switch kMsg.String() {
	case "enter":
		return c, c.submitCmd()
	case "esc":
		return c, func() tea.Msg { return confirmCancelledMsg{} }
	}
	// All other printable chars feed the note input.
	var cmd tea.Cmd
	c.note, cmd = c.note.Update(kMsg)
	return c, cmd
}

type confirmCancelledMsg struct{}

func (c confirmModel) submitCmd() tea.Cmd {
	note := strings.TrimSpace(c.note.Value())
	entity := c.entity
	a := c.appRef

	switch c.kind {
	case confirmLost:
		return func() tea.Msg {
			reqs := []app.ChangeStatusRequest{{
				EntityID: entity.EntityID,
				Status:   inventory.EntityStatusMissing,
				ActorID:  cli.GetActorUserID(context.Background()),
				Note:     note,
			}}
			results, err := a.MarkLost(context.Background(), reqs)
			if err != nil {
				return actionDoneMsg{err: fmt.Errorf("lost: %w", err)}
			}
			if len(results) == 0 {
				return actionDoneMsg{err: errors.New("lost: no result returned")}
			}
			return actionDoneMsg{result: results[0]}
		}

	case confirmReturn:
		return func() tea.Msg {
			reqs := []app.ChangeStatusRequest{{
				EntityID: entity.EntityID,
				ActorID:  cli.GetActorUserID(context.Background()),
				Note:     note,
			}}
			results, err := a.MarkReturned(context.Background(), reqs)
			if err != nil {
				return actionDoneMsg{err: fmt.Errorf("return: %w", err)}
			}
			if len(results) == 0 {
				return actionDoneMsg{err: errors.New("return: no result returned")}
			}
			return actionDoneMsg{result: results[0]}
		}

	case confirmFound:
		return func() tea.Msg {
			reqs := []app.ChangeStatusRequest{{
				EntityID: entity.EntityID,
				Status:   inventory.EntityStatusOk,
				ActorID:  cli.GetActorUserID(context.Background()),
				Note:     note,
			}}
			results, err := a.MarkFound(context.Background(), reqs)
			if err != nil {
				return actionDoneMsg{err: fmt.Errorf("found: %w", err)}
			}
			if len(results) == 0 {
				return actionDoneMsg{err: errors.New("found: no result returned")}
			}
			return actionDoneMsg{result: results[0]}
		}
	}
	return nil
}

func (c confirmModel) View(width int) string {
	prompt := fmt.Sprintf("mark %q as %s?", c.entity.FullPathDisplay, c.targetStatusLabel())
	noteView := c.note.View()
	help := c.st.Muted().Render("[enter] confirm  [esc] cancel")
	content := strings.Join([]string{prompt, noteView, help}, "\n")
	return lipgloss.NewStyle().Width(width).Render(content)
}
