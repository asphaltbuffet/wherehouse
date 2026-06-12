package tui

import (
	"context"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/styles"
)

// historyUIOverhead is the number of terminal rows used by title and help bar.
const historyUIOverhead = 4

type historyModel struct {
	entity   app.EntityResult
	viewport viewport.Model
	items    []app.HistoryResult
	appRef   App
	st       *styles.Styles
	ready    bool
	err      error
}

func newHistoryModel(entity app.EntityResult, a App, st *styles.Styles) historyModel {
	return historyModel{
		entity: entity,
		appRef: a,
		st:     st,
	}
}

func (h historyModel) loadCmd() tea.Cmd {
	entity := h.entity
	a := h.appRef
	return func() tea.Msg {
		items, err := a.GetHistory(context.Background(), app.GetHistoryRequest{
			EntityID: entity.EntityID,
		})
		return historyLoadedMsg{entity: entity, items: items, err: err}
	}
}

func (h historyModel) Update(msg tea.Msg) (historyModel, tea.Cmd) {
	switch msg := msg.(type) {
	case historyLoadedMsg:
		if msg.err != nil {
			h.err = msg.err
			return h, nil
		}
		h.items = msg.items
		h.viewport.SetContent(h.renderContent())
		h.ready = true
		return h, nil

	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "esc":
			return h, func() tea.Msg { return historyCancelledMsg{} }
		}
		var cmd tea.Cmd
		h.viewport, cmd = h.viewport.Update(msg)
		return h, cmd

	case tea.WindowSizeMsg:
		h.viewport = viewport.New(viewport.WithWidth(msg.Width), viewport.WithHeight(msg.Height-historyUIOverhead))
		if h.ready {
			h.viewport.SetContent(h.renderContent())
		}
		return h, nil
	}

	var cmd tea.Cmd
	h.viewport, cmd = h.viewport.Update(msg)
	return h, cmd
}

type historyCancelledMsg struct{}

func (h historyModel) renderContent() string {
	if h.err != nil {
		return fmt.Sprintf("error loading history: %v", h.err)
	}
	if len(h.items) == 0 {
		return "no history"
	}
	var sb strings.Builder
	for _, ev := range h.items {
		line := fmt.Sprintf("%s  %-30s  %s",
			ev.TimestampUTC,
			ev.EventType.String(),
			ev.ActorUserID,
		)
		if ev.Note != "" {
			line += "  " + ev.Note
		}
		sb.WriteString(line)
		sb.WriteString("\n")
	}
	return sb.String()
}

func (h historyModel) View(width, height int) string {
	title := h.st.TUIDetailLabel().Render("history: " + h.entity.FullPathDisplay)
	help := h.st.Muted().Render("[q/esc] back")
	content := h.viewport.View()
	if h.err != nil {
		content = h.st.DangerText().Render(h.err.Error())
	}
	return lipgloss.NewStyle().Width(width).Height(height).Render(
		strings.Join([]string{title, content, help}, "\n"),
	)
}
