package tui

import (
	"context"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"

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

func newHistoryModel(entity app.EntityResult, a App, st *styles.Styles, paneWidth, paneHeight int) historyModel {
	vpHeight := max(0, paneHeight-historyUIOverhead)
	return historyModel{
		entity:   entity,
		appRef:   a,
		st:       st,
		viewport: viewport.New(viewport.WithWidth(paneWidth), viewport.WithHeight(vpHeight)),
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
		var cmd tea.Cmd
		h.viewport, cmd = h.viewport.Update(msg)
		return h, cmd
	}

	var cmd tea.Cmd
	h.viewport, cmd = h.viewport.Update(msg)
	return h, cmd
}

func (h historyModel) renderContent() string {
	if h.err != nil {
		return fmt.Sprintf("error loading history: %v", h.err)
	}
	if len(h.items) == 0 {
		return "no history"
	}
	var sb strings.Builder
	for _, ev := range h.items {
		sb.WriteString(fmt.Sprintf("%s  %s\n  %s",
			ev.TimestampUTC,
			ev.EventType.String(),
			ev.ActorUserID,
		))
		if ev.Note != "" {
			sb.WriteString("  " + ev.Note)
		}
		sb.WriteString("\n\n")
	}
	return sb.String()
}

func (h historyModel) viewportView() string {
	if h.err != nil {
		return h.st.DangerText().Render(h.err.Error())
	}
	return h.viewport.View()
}
