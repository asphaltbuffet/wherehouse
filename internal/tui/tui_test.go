package tui_test

import (
	"context"
	"fmt"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/tui"
)

// fakeTUIApp implements tui.App with controllable return values.
type fakeTUIApp struct {
	roots    []app.EntityResult
	children map[string][]app.EntityResult
	err      error
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

func keyMsg(k string) tea.KeyPressMsg {
	return tea.KeyPressMsg{Text: k}
}

func entityResult(id, name, path string, hasChildren bool) app.EntityResult {
	return app.EntityResult{
		EntityID:        id,
		DisplayName:     name,
		FullPathDisplay: path,
		HasChildren:     hasChildren,
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
