package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/styles"
)

type formKind int

const (
	formAdd    formKind = iota // create entity as child of selected
	formLoan                   // mark selected entity as loaned
	formBorrow                 // create borrowed entity as child of selected
)

type formModel struct {
	kind    formKind
	inputs  []textinput.Model
	labels  []string // display label for each input (parallel to inputs)
	focused int
	entity  app.EntityResult // pre-selected entity
	appRef  App
	st      *styles.Styles
	err     error
}

const notePlaceholder = "note (optional)"

var (
	formSubmitKey = key.NewBinding(key.WithKeys("enter"))
	formTabKey    = key.NewBinding(key.WithKeys("tab"))
)

func newFormModel(kind formKind, entity app.EntityResult, a App, st *styles.Styles) formModel {
	var inputs []textinput.Model

	var labels []string

	switch kind {
	case formAdd:
		parent := textinput.New()
		parent.Placeholder = "parent path"
		parent.SetValue(entity.FullPathDisplay)
		parent.Focus()
		name := textinput.New()
		name.Placeholder = "name (required)"
		note := textinput.New()
		note.Placeholder = notePlaceholder
		inputs = []textinput.Model{parent, name, note}
		labels = []string{"Parent", "Name", "Note"}

	case formLoan:
		to := textinput.New()
		to.Placeholder = "loaned to (required)"
		to.Focus()
		note := textinput.New()
		note.Placeholder = notePlaceholder
		inputs = []textinput.Model{to, note}
		labels = []string{"Loaned to", "Note"}

	case formBorrow:
		parent := textinput.New()
		parent.Placeholder = "parent path"
		parent.SetValue(entity.FullPathDisplay)
		parent.Focus()
		name := textinput.New()
		name.Placeholder = "name (required)"
		from := textinput.New()
		from.Placeholder = "borrowed from (required)"
		note := textinput.New()
		note.Placeholder = notePlaceholder
		inputs = []textinput.Model{parent, name, from, note}
		labels = []string{"Parent", "Name", "Borrowed from", "Note"}
	}

	return formModel{
		kind:    kind,
		inputs:  inputs,
		labels:  labels,
		focused: 0,
		entity:  entity,
		appRef:  a,
		st:      st,
	}
}

// kindName returns a string label for test assertions.
func (f formModel) kindName() string {
	switch f.kind {
	case formAdd:
		return "add"
	case formLoan:
		return "loan"
	case formBorrow:
		return "borrow"
	default:
		return ""
	}
}

// requiredFilled returns true when all required fields are non-empty.
func (f formModel) requiredFilled() bool {
	switch f.kind {
	case formAdd:
		// inputs[0]=parent, inputs[1]=name
		return strings.TrimSpace(f.inputs[0].Value()) != "" &&
			strings.TrimSpace(f.inputs[1].Value()) != ""
	case formLoan:
		// inputs[0]=loaned to
		return strings.TrimSpace(f.inputs[0].Value()) != ""
	case formBorrow:
		// inputs[0]=parent, inputs[1]=name, inputs[2]=borrowed from
		return strings.TrimSpace(f.inputs[0].Value()) != "" &&
			strings.TrimSpace(f.inputs[1].Value()) != "" &&
			strings.TrimSpace(f.inputs[2].Value()) != ""
	}
	return false
}

func (f formModel) Update(msg tea.Msg) (formModel, tea.Cmd) {
	kMsg, isKey := msg.(tea.KeyPressMsg)
	if isKey {
		switch {
		case key.Matches(kMsg, formTabKey):
			f.inputs[f.focused].Blur()
			f.focused = (f.focused + 1) % len(f.inputs)
			f.inputs[f.focused].Focus()
			return f, nil

		case key.Matches(kMsg, formSubmitKey):
			if f.requiredFilled() {
				return f, f.submitCmd()
			}
			return f, nil
		}

		var cmd tea.Cmd
		f.inputs[f.focused], cmd = f.inputs[f.focused].Update(kMsg)
		return f, cmd
	}

	var cmds []tea.Cmd
	for i := range f.inputs {
		var cmd tea.Cmd
		f.inputs[i], cmd = f.inputs[i].Update(msg)
		cmds = append(cmds, cmd)
	}
	return f, tea.Batch(cmds...)
}

func (f formModel) submitCmd() tea.Cmd {
	switch f.kind {
	case formAdd:
		// inputs[0]=parent, inputs[1]=name, inputs[2]=note
		parent := strings.TrimSpace(f.inputs[0].Value())
		name := strings.TrimSpace(f.inputs[1].Value())
		note := strings.TrimSpace(f.inputs[2].Value())
		a := f.appRef
		return func() tea.Msg {
			reqs := []app.CreateEntityRequest{{
				DisplayName: name,
				ParentPath:  parent,
				ActorID:     cli.GetActorUserID(context.Background()),
				Note:        note,
			}}
			results, err := a.CreateEntities(context.Background(), reqs)
			if err != nil {
				return actionDoneMsg{err: fmt.Errorf("add: %w", err)}
			}
			if len(results) == 0 {
				return actionDoneMsg{err: errors.New("add: no result returned")}
			}
			return actionDoneMsg{result: results[0]}
		}

	case formLoan:
		to := strings.TrimSpace(f.inputs[0].Value())
		note := strings.TrimSpace(f.inputs[1].Value())
		entity := f.entity
		a := f.appRef
		return func() tea.Msg {
			reqs := []app.ChangeStatusRequest{{
				EntityID:      entity.EntityID,
				StatusContext: to,
				ActorID:       cli.GetActorUserID(context.Background()),
				Note:          note,
			}}
			results, err := a.MarkLoaned(context.Background(), reqs)
			if err != nil {
				return actionDoneMsg{err: fmt.Errorf("loan: %w", err)}
			}
			if len(results) == 0 {
				return actionDoneMsg{err: errors.New("loan: no result returned")}
			}
			return actionDoneMsg{result: results[0]}
		}

	case formBorrow:
		// inputs[0]=parent, inputs[1]=name, inputs[2]=borrowed from, inputs[3]=note
		parent := strings.TrimSpace(f.inputs[0].Value())
		name := strings.TrimSpace(f.inputs[1].Value())
		from := strings.TrimSpace(f.inputs[2].Value())
		note := strings.TrimSpace(f.inputs[3].Value())
		a := f.appRef
		return func() tea.Msg {
			reqs := []app.BorrowEntityRequest{{
				DisplayName:   name,
				ParentPath:    parent,
				StatusContext: from,
				ActorID:       cli.GetActorUserID(context.Background()),
				Note:          note,
			}}
			results, err := a.BorrowEntities(context.Background(), reqs)
			if err != nil {
				return actionDoneMsg{err: fmt.Errorf("borrow: %w", err)}
			}
			if len(results) == 0 {
				return actionDoneMsg{err: errors.New("borrow: no result returned")}
			}
			return actionDoneMsg{result: results[0]}
		}
	}
	return nil
}

func (f formModel) View(width int) string {
	title := f.st.TUIDetailLabel().Render(f.kindName())
	var sb strings.Builder
	sb.WriteString(title)
	sb.WriteString("\n\n")
	for i, inp := range f.inputs {
		if i < len(f.labels) {
			sb.WriteString(f.st.Muted().Render(f.labels[i]))
			sb.WriteString("\n")
		}
		sb.WriteString(inp.View())
		sb.WriteString("\n")
	}
	if f.err != nil {
		sb.WriteString("\n")
		sb.WriteString(f.st.DangerText().Render(f.err.Error()))
	}
	help := f.st.Muted().Render("tab: next field  enter: submit  esc: cancel")
	return lipgloss.NewStyle().Width(width).Render(sb.String()) + "\n" + help
}
