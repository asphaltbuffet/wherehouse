package tui_test

import (
	"context"
	"fmt"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
	"github.com/asphaltbuffet/wherehouse/internal/tui"
)

// fakeTUIApp implements tui.App with controllable return values.
type fakeTUIApp struct {
	roots        []app.EntityResult
	children     map[string][]app.EntityResult
	err          error
	created      []app.EntityResult
	createErr    error
	loaned       []app.EntityResult
	loanErr      error
	borrowed     []app.EntityResult
	borrowErr    error
	lost         []app.EntityResult
	lostErr      error
	lastLostReqs []app.ChangeStatusRequest
	returned     []app.EntityResult
	returnErr    error
	found        []app.EntityResult
	foundErr     error
	history      []app.HistoryResult
	historyErr   error
	findResults  []app.FindResult
	findErr      error
}

func (f *fakeTUIApp) GetRootEntities(_ context.Context) ([]app.EntityResult, error) {
	return f.roots, f.err
}

func (f *fakeTUIApp) GetChildren(_ context.Context, parentID string) ([]app.EntityResult, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.children[parentID], nil
}

func (f *fakeTUIApp) CreateEntities(_ context.Context, _ []app.CreateEntityRequest) ([]app.EntityResult, error) {
	return f.created, f.createErr
}

func (f *fakeTUIApp) MarkLoaned(_ context.Context, _ []app.ChangeStatusRequest) ([]app.EntityResult, error) {
	return f.loaned, f.loanErr
}

func (f *fakeTUIApp) BorrowEntities(_ context.Context, _ []app.BorrowEntityRequest) ([]app.EntityResult, error) {
	return f.borrowed, f.borrowErr
}

func (f *fakeTUIApp) MarkLost(_ context.Context, reqs []app.ChangeStatusRequest) ([]app.EntityResult, error) {
	f.lastLostReqs = reqs
	return f.lost, f.lostErr
}

func (f *fakeTUIApp) MarkReturned(_ context.Context, _ []app.ChangeStatusRequest) ([]app.EntityResult, error) {
	return f.returned, f.returnErr
}

func (f *fakeTUIApp) MarkFound(_ context.Context, _ []app.ChangeStatusRequest) ([]app.EntityResult, error) {
	return f.found, f.foundErr
}

func (f *fakeTUIApp) GetHistory(_ context.Context, _ app.GetHistoryRequest) ([]app.HistoryResult, error) {
	return f.history, f.historyErr
}

func (f *fakeTUIApp) FindEntities(_ context.Context, _ app.FindEntitiesRequest) ([]app.FindResult, error) {
	return f.findResults, f.findErr
}

func keyMsg(k string) tea.KeyPressMsg {
	return tea.KeyPressMsg{Text: k}
}

// feedBatch executes cmd and feeds all resulting messages back into m.
// Handles both single-message cmds and tea.BatchMsg transparently.
func feedBatch(t *testing.T, m tui.Model, cmd tea.Cmd) tui.Model {
	t.Helper()
	if cmd == nil {
		return m
	}
	msg := cmd()
	switch msg := msg.(type) {
	case tea.BatchMsg:
		for _, c := range msg {
			if c == nil {
				continue
			}
			inner := c()
			updated, _ := m.Update(inner)
			m = updated.(tui.Model)
		}
	default:
		updated, _ := m.Update(msg)
		m = updated.(tui.Model)
	}
	return m
}

func entityResult(id, name, path string, hasChildren bool) app.EntityResult {
	return app.EntityResult{
		EntityID:        id,
		DisplayName:     name,
		FullPathDisplay: path,
		HasChildren:     hasChildren,
		Status:          inventory.EntityStatusOk,
	}
}

func entityResultWithStatus(id, name, path string, status inventory.EntityStatus, locked bool) app.EntityResult {
	return app.EntityResult{
		EntityID:        id,
		DisplayName:     name,
		FullPathDisplay: path,
		Status:          status,
		Locked:          locked,
	}
}

// loadedModel builds a model with root entities already loaded (bypassing Init).
func loadedModel(t *testing.T, roots []app.EntityResult) tui.Model {
	t.Helper()
	return loadedModelFake(t, &fakeTUIApp{roots: roots})
}

// loadedModelFake builds a model using a pre-configured fake app.
func loadedModelFake(t *testing.T, f *fakeTUIApp) tui.Model {
	t.Helper()
	m := tui.New(f)
	msg := m.Init()()
	updated, _ := m.Update(msg)
	return updated.(tui.Model)
}

// expandNode presses l on the current cursor position and waits for the
// treeExpandedMsg to come back, returning the settled model.
func expandNode(t *testing.T, m tui.Model) tui.Model {
	t.Helper()
	updated, cmd := m.Update(keyMsg("l"))
	m = updated.(tui.Model)
	if cmd != nil {
		childMsg := cmd()
		updated2, _ := m.Update(childMsg)
		m = updated2.(tui.Model)
	}
	return m
}

func TestModel_InitLoadsRootEntities(t *testing.T) {
	roots := []app.EntityResult{
		entityResult("e1", "Garage", "Garage", true),
		entityResult("e2", "Shelf", "Shelf", false),
	}
	f := &fakeTUIApp{roots: roots}

	m := tui.New(f)

	initCmd := m.Init()
	require.NotNil(t, initCmd)

	msg := initCmd()

	updatedModel, _ := m.Update(msg)
	updated := updatedModel.(tui.Model)

	assert.Equal(t, 2, updated.ItemCount())
	// At root with cursor on first entity.
	assert.Equal(t, "Garage", updated.CurrentPath())
}

func TestModel_ExpandCollapse(t *testing.T) {
	children := []app.EntityResult{
		entityResult("e10", "Toolbox", "Garage:Toolbox", false),
		entityResult("e11", "Ladder", "Garage:Ladder", false),
	}
	roots := []app.EntityResult{
		entityResult("e1", "Garage", "Garage", true),
		entityResult("e2", "Shelf", "Shelf", false),
	}
	f := &fakeTUIApp{
		roots:    roots,
		children: map[string][]app.EntityResult{"e1": children},
	}

	t.Run("l on entity with children expands it and shows children", func(t *testing.T) {
		m := loadedModelFake(t, f)
		// Cursor on Garage (index 0, HasChildren=true).
		m = expandNode(t, m)

		// Visible: Garage, Toolbox, Ladder, Shelf = 4 items.
		assert.Equal(t, 4, m.ItemCount())
		// Cursor remains on Garage.
		assert.Equal(t, "Garage", m.CurrentPath())
	})

	t.Run("l on already-expanded node collapses it", func(t *testing.T) {
		m := loadedModelFake(t, f)
		m = expandNode(t, m)
		require.Equal(t, 4, m.ItemCount())

		// Press l again — collapses Garage, children hidden.
		updated, cmd := m.Update(keyMsg("l"))
		m = updated.(tui.Model)
		assert.Nil(t, cmd)
		assert.Equal(t, 2, m.ItemCount())
	})

	t.Run("l on childless entity is no-op", func(t *testing.T) {
		m := loadedModelFake(t, f)
		// Move to Shelf (index 1, HasChildren=false).
		updated, _ := m.Update(keyMsg("j"))
		m2 := updated.(tui.Model)

		updated2, cmd := m2.Update(keyMsg("l"))
		assert.Nil(t, cmd)
		assert.Equal(t, 2, updated2.(tui.Model).ItemCount())
	})
}

func TestModel_CollapseNavigation(t *testing.T) {
	children := []app.EntityResult{
		entityResult("e10", "Toolbox", "Garage:Toolbox", false),
	}
	roots := []app.EntityResult{
		entityResult("e1", "Garage", "Garage", true),
		entityResult("e2", "Shelf", "Shelf", false),
	}
	f := &fakeTUIApp{
		roots:    roots,
		children: map[string][]app.EntityResult{"e1": children},
	}

	t.Run("h on expanded node collapses it", func(t *testing.T) {
		m := loadedModelFake(t, f)
		m = expandNode(t, m)
		require.Equal(t, 3, m.ItemCount())

		updated, cmd := m.Update(keyMsg("h"))
		m2 := updated.(tui.Model)
		assert.Nil(t, cmd)
		assert.Equal(t, 2, m2.ItemCount())
		assert.Equal(t, "Garage", m2.CurrentPath())
	})

	t.Run("h on collapsed child moves cursor to parent", func(t *testing.T) {
		m := loadedModelFake(t, f)
		// Expand Garage, move cursor to Toolbox.
		m = expandNode(t, m)
		updated, _ := m.Update(keyMsg("j"))
		m = updated.(tui.Model)
		require.Equal(t, "Garage:Toolbox", m.CurrentPath())

		// h moves cursor to parent (Garage), no load needed.
		updated2, cmd := m.Update(keyMsg("h"))
		m2 := updated2.(tui.Model)
		assert.Nil(t, cmd)
		assert.Equal(t, "Garage", m2.CurrentPath())
	})

	t.Run("h at root collapsed node is no-op", func(t *testing.T) {
		m := loadedModelFake(t, f)
		assert.Equal(t, "Garage", m.CurrentPath())

		updated, cmd := m.Update(keyMsg("h"))
		m2 := updated.(tui.Model)
		assert.Nil(t, cmd)
		assert.Equal(t, "Garage", m2.CurrentPath())
	})
}

func TestModel_QuitKeys(t *testing.T) {
	roots := []app.EntityResult{entityResult("e1", "Garage", "Garage", false)}

	for _, k := range []string{"q", "Q"} {
		t.Run(fmt.Sprintf("%s quits", k), func(t *testing.T) {
			m := loadedModel(t, roots)
			_, cmd := m.Update(keyMsg(k))
			require.NotNil(t, cmd)
			msg := cmd()
			assert.Equal(t, tea.QuitMsg{}, msg)
		})
	}

	t.Run("ctrl+c quits", func(t *testing.T) {
		m := loadedModel(t, roots)
		_, cmd := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
		require.NotNil(t, cmd)
		msg := cmd()
		assert.Equal(t, tea.QuitMsg{}, msg)
	})
}

func TestModel_ActionGating(t *testing.T) {
	t.Run("loan blocked when entity is locked sets errMsg", func(t *testing.T) {
		locked := entityResultWithStatus("e1", "Wrench", "Garage:Wrench", inventory.EntityStatusOk, true)
		f := &fakeTUIApp{roots: []app.EntityResult{locked}}
		m := loadedModelFake(t, f)

		updated, cmd := m.Update(keyMsg("L"))
		m2 := updated.(tui.Model)

		assert.Nil(t, cmd)
		assert.Equal(t, "browse", m2.Mode())
		assert.NotEmpty(t, m2.ErrMsg())
	})

	t.Run("loan blocked when entity is missing and locked sets errMsg", func(t *testing.T) {
		locked := entityResultWithStatus("e1", "Wrench", "Garage:Wrench", inventory.EntityStatusMissing, true)
		f := &fakeTUIApp{roots: []app.EntityResult{locked}}
		m := loadedModelFake(t, f)

		updated, cmd := m.Update(keyMsg("L"))
		m2 := updated.(tui.Model)

		assert.Nil(t, cmd)
		assert.Equal(t, "browse", m2.Mode())
		assert.NotEmpty(t, m2.ErrMsg())
	})

	t.Run("lost blocked when entity is not ok sets errMsg", func(t *testing.T) {
		missing := entityResultWithStatus("e1", "Wrench", "Garage:Wrench", inventory.EntityStatusMissing, false)
		f := &fakeTUIApp{roots: []app.EntityResult{missing}}
		m := loadedModelFake(t, f)

		updated, cmd := m.Update(keyMsg("x"))
		m2 := updated.(tui.Model)

		assert.Nil(t, cmd)
		assert.Equal(t, "browse", m2.Mode())
		assert.NotEmpty(t, m2.ErrMsg())
	})

	t.Run("lost blocked when entity is ok but locked sets errMsg", func(t *testing.T) {
		locked := entityResultWithStatus("e1", "Wrench", "Garage:Wrench", inventory.EntityStatusOk, true)
		f := &fakeTUIApp{roots: []app.EntityResult{locked}}
		m := loadedModelFake(t, f)

		updated, cmd := m.Update(keyMsg("x"))
		m2 := updated.(tui.Model)

		assert.Nil(t, cmd)
		assert.Equal(t, "browse", m2.Mode())
		assert.NotEmpty(t, m2.ErrMsg())
	})

	t.Run("found blocked when entity is not missing sets errMsg", func(t *testing.T) {
		ok := entityResultWithStatus("e1", "Wrench", "Garage:Wrench", inventory.EntityStatusOk, false)
		f := &fakeTUIApp{roots: []app.EntityResult{ok}}
		m := loadedModelFake(t, f)

		updated, cmd := m.Update(keyMsg("f"))
		m2 := updated.(tui.Model)

		assert.Nil(t, cmd)
		assert.Equal(t, "browse", m2.Mode())
		assert.NotEmpty(t, m2.ErrMsg())
	})

	t.Run("return blocked when entity is ok sets errMsg", func(t *testing.T) {
		ok := entityResultWithStatus("e1", "Wrench", "Garage:Wrench", inventory.EntityStatusOk, false)
		f := &fakeTUIApp{roots: []app.EntityResult{ok}}
		m := loadedModelFake(t, f)

		updated, cmd := m.Update(keyMsg("r"))
		m2 := updated.(tui.Model)

		assert.Nil(t, cmd)
		assert.Equal(t, "browse", m2.Mode())
		assert.NotEmpty(t, m2.ErrMsg())
	})

	t.Run("add blocked when entity is discrete sets errMsg", func(t *testing.T) {
		discrete := app.EntityResult{
			EntityID:        "e1",
			DisplayName:     "Box",
			FullPathDisplay: "Garage:Box",
			Status:          inventory.EntityStatusOk,
			Discrete:        true,
		}
		f := &fakeTUIApp{roots: []app.EntityResult{discrete}}
		m := loadedModelFake(t, f)

		updated, cmd := m.Update(keyMsg("a"))
		m2 := updated.(tui.Model)

		assert.Nil(t, cmd)
		assert.Equal(t, "browse", m2.Mode())
		assert.NotEmpty(t, m2.ErrMsg())
	})

	t.Run("errMsg cleared on next navigation key", func(t *testing.T) {
		locked := entityResultWithStatus("e1", "Wrench", "Garage:Wrench", inventory.EntityStatusOk, true)
		shelf := entityResultWithStatus("e2", "Shelf", "Shelf", inventory.EntityStatusOk, false)
		f := &fakeTUIApp{roots: []app.EntityResult{locked, shelf}}
		m := loadedModelFake(t, f)

		updated, _ := m.Update(keyMsg("L"))
		m2 := updated.(tui.Model)
		require.NotEmpty(t, m2.ErrMsg())

		updated2, _ := m2.Update(keyMsg("j"))
		assert.Empty(t, updated2.(tui.Model).ErrMsg())
	})
}

func TestModel_ConfirmMode(t *testing.T) {
	okEntity := entityResultWithStatus("e1", "Wrench", "Garage:Wrench", inventory.EntityStatusOk, false)
	missingEntity := entityResultWithStatus("e2", "Hammer", "Garage:Hammer", inventory.EntityStatusMissing, false)
	loanedEntity := entityResultWithStatus("e3", "Drill", "Garage:Drill", inventory.EntityStatusLoaned, false)

	t.Run("x on ok entity opens modeConfirm", func(t *testing.T) {
		f := &fakeTUIApp{roots: []app.EntityResult{okEntity}}
		m := loadedModelFake(t, f)

		updated, cmd := m.Update(keyMsg("x"))
		m2 := updated.(tui.Model)

		assert.Equal(t, "confirm", m2.Mode())
		assert.Nil(t, cmd)
	})

	t.Run("r on loaned entity opens modeConfirm", func(t *testing.T) {
		f := &fakeTUIApp{roots: []app.EntityResult{loanedEntity}}
		m := loadedModelFake(t, f)

		updated, cmd := m.Update(keyMsg("r"))
		m2 := updated.(tui.Model)

		assert.Equal(t, "confirm", m2.Mode())
		assert.Nil(t, cmd)
	})

	t.Run("f on missing entity opens modeConfirm", func(t *testing.T) {
		f := &fakeTUIApp{roots: []app.EntityResult{missingEntity}}
		m := loadedModelFake(t, f)

		updated, cmd := m.Update(keyMsg("f"))
		m2 := updated.(tui.Model)

		assert.Equal(t, "confirm", m2.Mode())
		assert.Nil(t, cmd)
	})

	t.Run("n goes into note field, does not cancel", func(t *testing.T) {
		f := &fakeTUIApp{roots: []app.EntityResult{okEntity}}
		m := loadedModelFake(t, f)

		updated, _ := m.Update(keyMsg("x"))
		require.Equal(t, "confirm", updated.(tui.Model).Mode())

		updated2, _ := updated.(tui.Model).Update(keyMsg("n"))
		assert.Equal(t, "confirm", updated2.(tui.Model).Mode())
		assert.Equal(t, "n", updated2.(tui.Model).ConfirmNote())
	})

	t.Run("esc cancels confirm and returns to browse", func(t *testing.T) {
		f := &fakeTUIApp{roots: []app.EntityResult{okEntity}}
		m := loadedModelFake(t, f)

		updated, _ := m.Update(keyMsg("x"))
		require.Equal(t, "confirm", updated.(tui.Model).Mode())

		updated2, cancelCmd := updated.(tui.Model).Update(keyMsg("esc"))
		require.NotNil(t, cancelCmd)
		updated3, _ := updated2.(tui.Model).Update(cancelCmd())
		assert.Equal(t, "browse", updated3.(tui.Model).Mode())
	})

	t.Run("typing a letter puts it in the note field", func(t *testing.T) {
		f := &fakeTUIApp{roots: []app.EntityResult{okEntity}}
		m := loadedModelFake(t, f)

		updated, _ := m.Update(keyMsg("x"))
		require.Equal(t, "confirm", updated.(tui.Model).Mode())

		updated2, _ := updated.(tui.Model).Update(keyMsg("g"))
		assert.Equal(t, "g", updated2.(tui.Model).ConfirmNote())
	})

	t.Run("y goes into note field, does not submit", func(t *testing.T) {
		f := &fakeTUIApp{roots: []app.EntityResult{okEntity}}
		m := loadedModelFake(t, f)

		updated, _ := m.Update(keyMsg("x"))
		require.Equal(t, "confirm", updated.(tui.Model).Mode())

		updated2, _ := updated.(tui.Model).Update(keyMsg("y"))
		assert.Equal(t, "confirm", updated2.(tui.Model).Mode())
		assert.Equal(t, "y", updated2.(tui.Model).ConfirmNote())
	})

	t.Run("enter submits note content to app", func(t *testing.T) {
		refreshed := entityResultWithStatus("e1", "Wrench", "Garage:Wrench", inventory.EntityStatusMissing, false)
		f := &fakeTUIApp{
			roots: []app.EntityResult{okEntity},
			lost:  []app.EntityResult{refreshed},
		}
		m := loadedModelFake(t, f)

		updated, _ := m.Update(keyMsg("x"))
		require.Equal(t, "confirm", updated.(tui.Model).Mode())
		updated, _ = updated.(tui.Model).Update(keyMsg("m"))
		updated, _ = updated.(tui.Model).Update(keyMsg("i"))
		updated, _ = updated.(tui.Model).Update(keyMsg("a"))

		_, submitCmd := updated.(tui.Model).Update(keyMsg("enter"))
		require.NotNil(t, submitCmd)
		submitCmd()

		require.Len(t, f.lastLostReqs, 1)
		assert.Equal(t, "mia", f.lastLostReqs[0].Note)
	})

	t.Run("enter submits lost and refreshes tree", func(t *testing.T) {
		refreshed := entityResultWithStatus("e1", "Wrench", "Garage:Wrench", inventory.EntityStatusMissing, false)
		f := &fakeTUIApp{
			roots: []app.EntityResult{okEntity},
			lost:  []app.EntityResult{refreshed},
		}
		m := loadedModelFake(t, f)

		updated, _ := m.Update(keyMsg("x"))
		require.Equal(t, "confirm", updated.(tui.Model).Mode())

		updated2, submitCmd := updated.(tui.Model).Update(keyMsg("enter"))
		require.NotNil(t, submitCmd)

		actionMsg := submitCmd()
		updated3, refreshCmd := updated2.(tui.Model).Update(actionMsg)
		assert.Equal(t, "browse", updated3.(tui.Model).Mode())
		require.NotNil(t, refreshCmd)

		// Execute refreshCmd → treeRefreshMsg with updated entity.
		refreshMsg := refreshCmd()
		updated4, _ := updated3.(tui.Model).Update(refreshMsg)
		assert.Equal(t, "browse", updated4.(tui.Model).Mode())
		// Root level still has 1 entity.
		assert.Equal(t, 1, updated4.(tui.Model).ItemCount())
	})
}

func TestModel_FormMode(t *testing.T) {
	okEntity := entityResultWithStatus("e1", "Garage", "Garage", inventory.EntityStatusOk, false)

	t.Run("a on non-discrete entity opens modeForm with add kind", func(t *testing.T) {
		f := &fakeTUIApp{roots: []app.EntityResult{okEntity}}
		m := loadedModelFake(t, f)

		updated, cmd := m.Update(keyMsg("a"))
		m2 := updated.(tui.Model)

		assert.Equal(t, "form", m2.Mode())
		assert.Equal(t, "add", m2.FormKind())
		assert.Nil(t, cmd)
	})

	t.Run("L on ok unlocked entity opens modeForm with loan kind", func(t *testing.T) {
		f := &fakeTUIApp{roots: []app.EntityResult{okEntity}}
		m := loadedModelFake(t, f)

		updated, cmd := m.Update(keyMsg("L"))
		m2 := updated.(tui.Model)

		assert.Equal(t, "form", m2.Mode())
		assert.Equal(t, "loan", m2.FormKind())
		assert.Nil(t, cmd)
	})

	t.Run("b opens modeForm with borrow kind", func(t *testing.T) {
		f := &fakeTUIApp{roots: []app.EntityResult{okEntity}}
		m := loadedModelFake(t, f)

		updated, cmd := m.Update(keyMsg("b"))
		m2 := updated.(tui.Model)

		assert.Equal(t, "form", m2.Mode())
		assert.Equal(t, "borrow", m2.FormKind())
		assert.Nil(t, cmd)
	})

	t.Run("esc cancels form and returns to browse", func(t *testing.T) {
		f := &fakeTUIApp{roots: []app.EntityResult{okEntity}}
		m := loadedModelFake(t, f)

		updated, _ := m.Update(keyMsg("a"))
		require.Equal(t, "form", updated.(tui.Model).Mode())

		updated2, _ := updated.(tui.Model).Update(keyMsg("esc"))
		assert.Equal(t, "browse", updated2.(tui.Model).Mode())
	})
}

func TestModel_HistoryRendersEvents(t *testing.T) {
	entity := entityResultWithStatus("e1", "Wrench", "Garage:Wrench", inventory.EntityStatusOk, false)
	f := &fakeTUIApp{
		roots: []app.EntityResult{entity},
		history: []app.HistoryResult{
			{
				EventID:      34,
				ActorUserID:  "alice",
				EventType:    inventory.EntityCreatedEvent,
				TimestampUTC: "2026-05-26T02:40:58Z",
			},
		},
	}
	m := loadedModelFake(t, f)

	sized, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = sized.(tui.Model)

	afterOpen, loadCmd := m.Update(keyMsg("H"))
	m2 := afterOpen.(tui.Model)
	require.Equal(t, "history", m2.RightPane())

	histMsg := loadCmd()
	afterLoad, _ := m2.Update(histMsg)
	m3 := afterLoad.(tui.Model)

	view := m3.View().Content
	assert.Contains(t, view, "alice", "rendered history should include actor name")
	assert.Contains(t, view, "2026-05-26T02:40:58Z", "rendered history should include timestamp")
	assert.Contains(t, view, "entity.created", "rendered history should include event type")
}

func TestModel_HistoryPane(t *testing.T) {
	entity := entityResultWithStatus("e1", "Wrench", "Garage:Wrench", inventory.EntityStatusOk, false)

	t.Run("H opens history right pane and fires load cmd", func(t *testing.T) {
		f := &fakeTUIApp{
			roots:   []app.EntityResult{entity},
			history: []app.HistoryResult{{EventID: 1, ActorUserID: "alice"}},
		}
		m := loadedModelFake(t, f)

		updated, loadCmd := m.Update(keyMsg("H"))
		m2 := updated.(tui.Model)

		assert.Equal(t, "history", m2.RightPane())
		assert.Equal(t, "browse", m2.Mode())
		require.NotNil(t, loadCmd)

		histMsg := loadCmd()
		updated2, _ := m2.Update(histMsg)
		assert.Equal(t, "history", updated2.(tui.Model).RightPane())
	})

	t.Run("H again hides history pane", func(t *testing.T) {
		f := &fakeTUIApp{
			roots:   []app.EntityResult{entity},
			history: []app.HistoryResult{},
		}
		m := loadedModelFake(t, f)

		updated, loadCmd := m.Update(keyMsg("H"))
		require.Equal(t, "history", updated.(tui.Model).RightPane())
		histMsg := loadCmd()
		updated2, _ := updated.(tui.Model).Update(histMsg)

		updated3, _ := updated2.(tui.Model).Update(keyMsg("H"))
		assert.Equal(t, "hidden", updated3.(tui.Model).RightPane())
		assert.Equal(t, "browse", updated3.(tui.Model).Mode())
	})
}

func TestModel_ScryMode(t *testing.T) {
	entity := entityResultWithStatus("e1", "Wrench", "Garage:Wrench", inventory.EntityStatusOk, false)

	t.Run("s opens modeScry", func(t *testing.T) {
		f := &fakeTUIApp{roots: []app.EntityResult{entity}}
		m := loadedModelFake(t, f)

		updated, _ := m.Update(keyMsg("s"))
		assert.Equal(t, "scry", updated.(tui.Model).Mode())
	})

	t.Run("esc from scry returns to browse", func(t *testing.T) {
		f := &fakeTUIApp{roots: []app.EntityResult{entity}}
		m := loadedModelFake(t, f)

		updated, _ := m.Update(keyMsg("s"))
		require.Equal(t, "scry", updated.(tui.Model).Mode())

		updated2, _ := updated.(tui.Model).Update(keyMsg("esc"))
		assert.Equal(t, "browse", updated2.(tui.Model).Mode())
	})
}

func TestModel_ScryShowsResults(t *testing.T) {
	drillBit := entityResult("e1", "Drill Bit", "Garage:Drill Bit", false)
	f := &fakeTUIApp{
		roots:       []app.EntityResult{drillBit},
		findResults: []app.FindResult{{Entity: drillBit, Distance: 0}},
	}
	m := loadedModelFake(t, f)

	sized, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = sized.(tui.Model)

	inScry, _ := m.Update(keyMsg("s"))
	m = inScry.(tui.Model)
	require.Equal(t, "scry", m.Mode())

	_, batchCmd := m.Update(keyMsg("d"))
	require.NotNil(t, batchCmd)
	m = feedBatch(t, m, batchCmd)

	assert.Contains(t, m.View().Content, "Garage:Drill Bit")
}

func TestModel_ScryNavigate(t *testing.T) {
	drillBit := entityResult("e1", "Drill Bit", "Garage:Drill Bit", false)
	f := &fakeTUIApp{
		roots:       []app.EntityResult{drillBit},
		findResults: []app.FindResult{{Entity: drillBit, Distance: 0}},
	}
	m := loadedModelFake(t, f)

	sized, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = sized.(tui.Model)

	inScry, _ := m.Update(keyMsg("s"))
	m = inScry.(tui.Model)

	_, batchCmd := m.Update(keyMsg("d"))
	m = feedBatch(t, m, batchCmd)

	afterEnter, navigateCmd := m.Update(keyMsg("enter"))
	require.NotNil(t, navigateCmd, "enter with results should return a navigate cmd")

	navigateMsg := navigateCmd()
	afterNavigate, _ := afterEnter.(tui.Model).Update(navigateMsg)
	assert.Equal(t, "browse", afterNavigate.(tui.Model).Mode())
	// Target entity is already visible in the tree; cursor should be on it.
	assert.Equal(t, "Garage:Drill Bit", afterNavigate.(tui.Model).CurrentPath())
}

func TestModel_ScryEnterNoResults(t *testing.T) {
	entity := entityResultWithStatus("e1", "Wrench", "Garage:Wrench", inventory.EntityStatusOk, false)
	f := &fakeTUIApp{roots: []app.EntityResult{entity}}
	m := loadedModelFake(t, f)

	inScry, _ := m.Update(keyMsg("s"))
	require.Equal(t, "scry", inScry.(tui.Model).Mode())

	updated, _ := inScry.(tui.Model).Update(keyMsg("enter"))
	assert.Equal(t, "scry", updated.(tui.Model).Mode())
}

func TestModel_ScryTabMovesToList(t *testing.T) {
	drillBit := entityResult("e1", "Drill Bit", "Garage:Drill Bit", false)
	f := &fakeTUIApp{
		roots:       []app.EntityResult{drillBit},
		findResults: []app.FindResult{{Entity: drillBit, Distance: 0}},
	}
	m := loadedModelFake(t, f)

	sized, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = sized.(tui.Model)

	inScry, _ := m.Update(keyMsg("s"))
	m = inScry.(tui.Model)
	require.Equal(t, "scry", m.Mode())

	// Load results.
	_, batchCmd := m.Update(keyMsg("d"))
	m = feedBatch(t, m, batchCmd)
	assert.Contains(t, m.View().Content, "Garage:Drill Bit")

	// Tab moves focus to the list; enter from list focus still navigates.
	afterTab, _ := m.Update(keyMsg("tab"))
	m = afterTab.(tui.Model)

	afterEnter, navigateCmd := m.Update(keyMsg("enter"))
	require.NotNil(t, navigateCmd, "enter from list focus should produce navigate cmd")

	navigateMsg := navigateCmd()
	afterNavigate, _ := afterEnter.(tui.Model).Update(navigateMsg)
	assert.Equal(t, "browse", afterNavigate.(tui.Model).Mode())
	assert.Equal(t, "Garage:Drill Bit", afterNavigate.(tui.Model).CurrentPath())
}

func TestModel_CursorNavigation(t *testing.T) {
	roots := []app.EntityResult{
		entityResult("e1", "Garage", "Garage", true),
		entityResult("e2", "Shelf", "Shelf", false),
		entityResult("e3", "Workshop", "Workshop", false),
	}

	t.Run("j moves cursor down", func(t *testing.T) {
		m := loadedModel(t, roots)
		assert.Equal(t, 0, m.CursorIndex())

		updated, _ := m.Update(keyMsg("j"))
		assert.Equal(t, 1, updated.(tui.Model).CursorIndex())
	})

	t.Run("k moves cursor up", func(t *testing.T) {
		m := loadedModel(t, roots)
		updated, _ := m.Update(keyMsg("j"))
		updated, _ = updated.(tui.Model).Update(keyMsg("k"))
		assert.Equal(t, 0, updated.(tui.Model).CursorIndex())
	})

	t.Run("k at top is clamped", func(t *testing.T) {
		m := loadedModel(t, roots)
		updated, _ := m.Update(keyMsg("k"))
		assert.Equal(t, 0, updated.(tui.Model).CursorIndex())
	})

	t.Run("j at bottom is clamped", func(t *testing.T) {
		m := loadedModel(t, roots)
		updated, _ := m.Update(keyMsg("j"))
		updated, _ = updated.(tui.Model).Update(keyMsg("j"))
		updated, _ = updated.(tui.Model).Update(keyMsg("j")) // past end
		assert.Equal(t, 2, updated.(tui.Model).CursorIndex())
	})
}

func TestModel_TreeExpandPreservesState(t *testing.T) {
	// Expanding one node must not collapse or disturb siblings.
	garage := entityResult("e1", "Garage", "Garage", true)
	shelf := entityResult("e2", "Shelf", "Shelf", true)
	garageKids := []app.EntityResult{
		entityResult("e10", "Toolbox", "Garage:Toolbox", false),
	}
	shelfKids := []app.EntityResult{
		entityResult("e20", "Book", "Shelf:Book", false),
	}
	f := &fakeTUIApp{
		roots: []app.EntityResult{garage, shelf},
		children: map[string][]app.EntityResult{
			"e1": garageKids,
			"e2": shelfKids,
		},
	}

	m := loadedModelFake(t, f)

	// Expand Garage.
	m = expandNode(t, m)
	require.Equal(t, 3, m.ItemCount()) // Garage, Toolbox, Shelf

	// Move to Shelf (index 2) and expand it.
	updated, _ := m.Update(keyMsg("j"))
	m = updated.(tui.Model)
	updated, _ = m.Update(keyMsg("j"))
	m = updated.(tui.Model)
	require.Equal(t, "Shelf", m.CurrentPath())
	m = expandNode(t, m)

	// Both Garage (and its child) and Shelf (and its child) must be visible.
	// Garage, Toolbox, Shelf, Book = 4.
	assert.Equal(t, 4, m.ItemCount())
}
