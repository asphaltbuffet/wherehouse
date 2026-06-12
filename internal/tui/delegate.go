package tui

import (
	"fmt"
	"io"
	"strings"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/asphaltbuffet/wherehouse/internal/inventory"
	"github.com/asphaltbuffet/wherehouse/internal/styles"
)

const chevron = "▶"

type delegate struct {
	selected lipgloss.Style
	normal   lipgloss.Style
	st       *styles.Styles
}

func newDelegate(st *styles.Styles) delegate {
	return delegate{
		selected: st.TUISelected(),
		normal:   lipgloss.NewStyle(),
		st:       st,
	}
}

func (d delegate) Height() int                             { return 1 }
func (d delegate) Spacing() int                            { return 0 }
func (d delegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d delegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(item)
	if !ok {
		return
	}

	rendered := d.st.AccentText().Render(chevron)
	prefix := rendered
	if !i.result.HasChildren {
		prefix = strings.Repeat(" ", lipgloss.Width(rendered))
	}

	name := i.result.DisplayName
	if index == m.Index() {
		name = d.selected.Render(name)
	}

	statusTag := ""
	if i.result.Status != inventory.EntityStatusOk {
		statusTag = " " + d.st.Muted().Render(fmt.Sprintf("[%s]", i.result.Status.String()))
	}

	fmt.Fprintln(w, prefix+" "+name+statusTag)
}
