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
	roots       []app.EntityResult
	children    map[string][]app.EntityResult
	err         error
	created     []app.EntityResult
	createErr   error
	loaned      []app.EntityResult
	loanErr     error
	borrowed    []app.EntityResult
	borrowErr   error
	lost        []app.EntityResult
	lostErr     error
	returned    []app.EntityResult
	returnErr   error
	found       []app.EntityResult
	foundErr    error
	history     []app.HistoryResult
	historyErr  error
	findResults []app.FindResult
	findErr     error
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

func (f *fakeTUIApp) MarkLost(_ context.Context, _ []app.ChangeStatusRequest) ([]app.EntityResult, error) {
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

func TestModel_InitLoadsRootEntities(t *testing.T) {
	roots := []app.EntityResult{
		entityResult("e1", "Garage", "Garage", true),
		entityResult("e2", "Shelf", "Shelf", false),
	}
	f := &fakeTUIApp{roots: roots}

	m := tui.New(f)

	// Init returns a Cmd that fetches root entities; simulate receiving the result.
	initCmd := m.Init()
	require.NotNil(t, initCmd)

	// Execute the command to get the message it produces.
	msg := initCmd()

	updatedModel, _ := m.Update(msg)
	updated := updatedModel.(tui.Model)

	assert.Equal(t, 2, updated.ItemCount())
	assert.Equal(t, "wherehouse", updated.CurrentPath())
}

func TestModel_DrillDown(t *testing.T) {
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

	t.Run("l on entity with children loads children and updates path", func(t *testing.T) {
		m := loadedModelFake(t, f)

		// Press l to drill into Garage (index 0, HasChildren=true).
		drillCmd := func() tea.Model {
			updated, cmd := m.Update(keyMsg("l"))
			require.NotNil(t, cmd)
			childMsg := cmd()
			updated, _ = updated.(tui.Model).Update(childMsg)
			return updated
		}()

		assert.Equal(t, "Garage", drillCmd.(tui.Model).CurrentPath())
		assert.Equal(t, 2, drillCmd.(tui.Model).ItemCount())
	})

	t.Run("l on childless entity is no-op", func(t *testing.T) {
		m := loadedModelFake(t, f)
		// Move to Shelf (index 1, HasChildren=false).
		updated, _ := m.Update(keyMsg("j"))
		m2 := updated.(tui.Model)

		updated2, cmd := m2.Update(keyMsg("l"))
		assert.Nil(t, cmd)
		assert.Equal(t, "wherehouse", updated2.(tui.Model).CurrentPath())
		assert.Equal(t, 2, updated2.(tui.Model).ItemCount())
	})
}

func TestModel_DrillUp(t *testing.T) {
	grandchildren := []app.EntityResult{
		entityResult("e20", "Office", "Basement:Office", false),
	}
	children := []app.EntityResult{
		entityResult("e10", "Toolbox", "Garage:Toolbox", false),
		entityResult("e11", "Basement", "Basement", true),
	}
	roots := []app.EntityResult{
		entityResult("e1", "Garage", "Garage", true),
		entityResult("e2", "Basement", "Basement", true),
		entityResult("e3", "Shelf", "Shelf", false),
	}
	f := &fakeTUIApp{
		roots: roots,
		children: map[string][]app.EntityResult{
			"e1":  children,
			"e11": grandchildren,
		},
	}

	t.Run("h after single drill-down returns to root", func(t *testing.T) {
		m := loadedModelFake(t, f)

		// Drill down into Garage (index 0).
		_, cmd := m.Update(keyMsg("l"))
		drilled, _ := m.Update(cmd())
		require.Equal(t, "Garage", drilled.(tui.Model).CurrentPath())

		// Drill up.
		_, upCmd := drilled.(tui.Model).Update(keyMsg("h"))
		require.NotNil(t, upCmd)
		back, _ := drilled.(tui.Model).Update(upCmd())

		assert.Equal(t, "wherehouse", back.(tui.Model).CurrentPath())
		assert.Equal(t, 3, back.(tui.Model).ItemCount())
	})

	t.Run("h after two drill-downs returns to intermediate level not root", func(t *testing.T) {
		m := loadedModelFake(t, f)

		// Drill into Garage children.
		_, cmd := m.Update(keyMsg("l"))
		depth1, _ := m.Update(cmd())
		require.Equal(t, "Garage", depth1.(tui.Model).CurrentPath())

		// Select the Basement child (index 1) and drill in.
		depth1, _ = depth1.(tui.Model).Update(keyMsg("j"))
		_, cmd2 := depth1.(tui.Model).Update(keyMsg("l"))
		depth2, _ := depth1.(tui.Model).Update(cmd2())
		require.Equal(t, "Basement", depth2.(tui.Model).CurrentPath())

		// Press h — must go back to Garage children, not root.
		_, upCmd := depth2.(tui.Model).Update(keyMsg("h"))
		require.NotNil(t, upCmd)
		back, _ := depth2.(tui.Model).Update(upCmd())

		assert.Equal(t, "Garage", back.(tui.Model).CurrentPath())
		assert.Equal(t, 2, back.(tui.Model).ItemCount())
	})

	t.Run("h twice from two levels deep reaches root", func(t *testing.T) {
		m := loadedModelFake(t, f)

		// Drill Garage → Basement child.
		_, cmd := m.Update(keyMsg("l"))
		depth1, _ := m.Update(cmd())
		depth1, _ = depth1.(tui.Model).Update(keyMsg("j"))
		_, cmd2 := depth1.(tui.Model).Update(keyMsg("l"))
		depth2, _ := depth1.(tui.Model).Update(cmd2())

		// First h → back to Garage.
		_, upCmd := depth2.(tui.Model).Update(keyMsg("h"))
		mid, _ := depth2.(tui.Model).Update(upCmd())
		require.Equal(t, "Garage", mid.(tui.Model).CurrentPath())

		// Second h → back to root.
		_, upCmd2 := mid.(tui.Model).Update(keyMsg("h"))
		require.NotNil(t, upCmd2)
		root, _ := mid.(tui.Model).Update(upCmd2())
		assert.Equal(t, "wherehouse", root.(tui.Model).CurrentPath())
		assert.Equal(t, 3, root.(tui.Model).ItemCount())
	})

	t.Run("h at root is no-op", func(t *testing.T) {
		m := loadedModelFake(t, f)
		updated, cmd := m.Update(keyMsg("h"))
		assert.Nil(t, cmd)
		assert.Equal(t, "wherehouse", updated.(tui.Model).CurrentPath())
	})
}

func TestModel_QuitKeys(t *testing.T) {
	roots := []app.EntityResult{entityResult("e1", "Garage", "Garage", false)}

	for _, key := range []string{"q", "Q"} {
		t.Run(fmt.Sprintf("%s quits", key), func(t *testing.T) {
			m := loadedModel(t, roots)
			_, cmd := m.Update(keyMsg(key))
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

		// Trigger an errMsg.
		updated, _ := m.Update(keyMsg("L"))
		m2 := updated.(tui.Model)
		require.NotEmpty(t, m2.ErrMsg())

		// Navigate down — errMsg should clear.
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

	t.Run("n cancels confirm and returns to browse", func(t *testing.T) {
		f := &fakeTUIApp{roots: []app.EntityResult{okEntity}}
		m := loadedModelFake(t, f)

		updated, _ := m.Update(keyMsg("x"))
		require.Equal(t, "confirm", updated.(tui.Model).Mode())

		updated2, _ := updated.(tui.Model).Update(keyMsg("n"))
		assert.Equal(t, "browse", updated2.(tui.Model).Mode())
	})

	t.Run("esc cancels confirm and returns to browse", func(t *testing.T) {
		f := &fakeTUIApp{roots: []app.EntityResult{okEntity}}
		m := loadedModelFake(t, f)

		updated, _ := m.Update(keyMsg("x"))
		require.Equal(t, "confirm", updated.(tui.Model).Mode())

		updated2, _ := updated.(tui.Model).Update(keyMsg("esc"))
		assert.Equal(t, "browse", updated2.(tui.Model).Mode())
	})

	t.Run("y submits lost and refreshes list", func(t *testing.T) {
		refreshed := entityResultWithStatus("e1", "Wrench", "Garage:Wrench", inventory.EntityStatusMissing, false)
		f := &fakeTUIApp{
			roots: []app.EntityResult{okEntity},
			lost:  []app.EntityResult{refreshed},
		}
		m := loadedModelFake(t, f)

		// Open confirm.
		updated, _ := m.Update(keyMsg("x"))
		require.Equal(t, "confirm", updated.(tui.Model).Mode())

		// Press y — fires submitCmd.
		updated2, submitCmd := updated.(tui.Model).Update(keyMsg("y"))
		require.NotNil(t, submitCmd)

		// Execute submitCmd → actionDoneMsg.
		actionMsg := submitCmd()
		updated3, refreshCmd := updated2.(tui.Model).Update(actionMsg)
		assert.Equal(t, "browse", updated3.(tui.Model).Mode())
		require.NotNil(t, refreshCmd)

		// Execute refreshCmd → childRefreshMsg.
		refreshMsg := refreshCmd()
		updated4, _ := updated3.(tui.Model).Update(refreshMsg)
		assert.Equal(t, "browse", updated4.(tui.Model).Mode())
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

	// Size the terminal so the viewport has non-zero dimensions.
	sized, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = sized.(tui.Model)

	// Open history and execute the load command.
	afterOpen, loadCmd := m.Update(keyMsg("H"))
	m2 := afterOpen.(tui.Model)
	require.Equal(t, "history", m2.Mode())

	histMsg := loadCmd()
	afterLoad, _ := m2.Update(histMsg)
	m3 := afterLoad.(tui.Model)

	view := m3.View().Content
	assert.Contains(t, view, "alice", "rendered history should include actor name")
	assert.Contains(t, view, "2026-05-26T02:40:58Z", "rendered history should include timestamp")
}

func TestModel_HistoryMode(t *testing.T) {
	entity := entityResultWithStatus("e1", "Wrench", "Garage:Wrench", inventory.EntityStatusOk, false)

	t.Run("H opens modeHistory and fires load cmd", func(t *testing.T) {
		f := &fakeTUIApp{
			roots:   []app.EntityResult{entity},
			history: []app.HistoryResult{{EventID: 1, ActorUserID: "alice"}},
		}
		m := loadedModelFake(t, f)

		updated, loadCmd := m.Update(keyMsg("H"))
		m2 := updated.(tui.Model)

		assert.Equal(t, "history", m2.Mode())
		require.NotNil(t, loadCmd)

		// Execute load cmd.
		histMsg := loadCmd()
		updated2, _ := m2.Update(histMsg)
		assert.Equal(t, "history", updated2.(tui.Model).Mode())
	})

	t.Run("esc from history returns to browse", func(t *testing.T) {
		f := &fakeTUIApp{
			roots:   []app.EntityResult{entity},
			history: []app.HistoryResult{},
		}
		m := loadedModelFake(t, f)

		updated, loadCmd := m.Update(keyMsg("H"))
		require.Equal(t, "history", updated.(tui.Model).Mode())
		histMsg := loadCmd()
		updated2, _ := updated.(tui.Model).Update(histMsg)

		updated3, _ := updated2.(tui.Model).Update(keyMsg("esc"))
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

func TestModel_ScryEnterNoResults(t *testing.T) {
	// Regression: pressing enter in scry with no matching results caused infinite
	// recursion (stack overflow) because updateScry re-routed scryNavigatedMsg
	// back through m.Update while still in modeScry.
	entity := entityResultWithStatus("e1", "Wrench", "Garage:Wrench", inventory.EntityStatusOk, false)
	f := &fakeTUIApp{roots: []app.EntityResult{entity}}
	m := loadedModelFake(t, f)

	// Open scry mode (findResults is empty — no matches for anything).
	inScry, _ := m.Update(keyMsg("s"))
	require.Equal(t, "scry", inScry.(tui.Model).Mode())

	// Press enter with an empty result list — must not panic or recurse.
	updated, _ := inScry.(tui.Model).Update(keyMsg("enter"))
	assert.Equal(t, "scry", updated.(tui.Model).Mode())
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
