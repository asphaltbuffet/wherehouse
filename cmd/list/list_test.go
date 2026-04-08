package list

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/config"
	"github.com/asphaltbuffet/wherehouse/internal/database"
	"github.com/asphaltbuffet/wherehouse/internal/nanoid"
)

// stripANSI removes ANSI escape codes from a string.
func stripANSI(s string) string {
	ansiRe := regexp.MustCompile("\x1b\\[[0-9;]*m")
	return ansiRe.ReplaceAllString(s, "")
}

// testFixture holds a test database with pre-populated data.
type testFixture struct {
	db          *database.Database
	ctx         context.Context
	garageID    string
	shelfAID    string
	workbenchID string
	drawer1ID   string
	officeID    string
	missingID   string
	item1ID     string // drill (garage)
	item2ID     string // hammer (garage)
	item3ID     string // sandpaper (shelfA)
	item4ID     string // chisel (workbench)
	item5ID     string // screwdriver (missing)
}

// setupListTest creates an in-memory DB with a tree of locations and items.
//
// Tree:
//
//	Garage (2 items: drill, hammer; 2 children: ShelfA, Workbench)
//	  ShelfA (1 item: sandpaper; 0 children)
//	  Workbench (1 item: chisel; 1 child: Drawer1)
//	    Drawer1 (0 items; 0 children)
//	Office (0 items; 0 children)
//	Missing [system] (1 item: screwdriver; 0 children)
func setupListTest(t *testing.T) testFixture {
	t.Helper()

	ctx := context.Background()
	db, err := database.Open(database.Config{
		Path:        ":memory:",
		BusyTimeout: database.DefaultBusyTimeout,
		AutoMigrate: true,
	})
	require.NoError(t, err)

	prefix := nanoid.MustNew()
	ts := "2025-01-01T00:00:00Z"

	f := testFixture{
		db:          db,
		ctx:         ctx,
		garageID:    nanoid.MustNew(),
		shelfAID:    nanoid.MustNew(),
		workbenchID: nanoid.MustNew(),
		drawer1ID:   nanoid.MustNew(),
		officeID:    nanoid.MustNew(),
		missingID:   nanoid.MustNew(),
		item1ID:     nanoid.MustNew(),
		item2ID:     nanoid.MustNew(),
		item3ID:     nanoid.MustNew(),
		item4ID:     nanoid.MustNew(),
		item5ID:     nanoid.MustNew(),
	}

	// Root locations
	require.NoError(t, db.CreateLocation(ctx, f.garageID, fmt.Sprintf("Garage-%s", prefix), nil, false, 0, ts))
	require.NoError(t, db.CreateLocation(ctx, f.officeID, fmt.Sprintf("Office-%s", prefix), nil, false, 0, ts))
	require.NoError(t, db.CreateLocation(ctx, f.missingID, fmt.Sprintf("Missing-%s", prefix), nil, true, 0, ts))

	// Child locations under Garage
	require.NoError(t, db.CreateLocation(ctx, f.shelfAID, fmt.Sprintf("ShelfA-%s", prefix), &f.garageID, false, 0, ts))
	require.NoError(
		t,
		db.CreateLocation(ctx, f.workbenchID, fmt.Sprintf("Workbench-%s", prefix), &f.garageID, false, 0, ts),
	)

	// Grandchild under Workbench
	require.NoError(
		t,
		db.CreateLocation(ctx, f.drawer1ID, fmt.Sprintf("Drawer1-%s", prefix), &f.workbenchID, false, 0, ts),
	)

	// Items
	require.NoError(t, db.CreateItem(ctx, f.item1ID, "drill", f.garageID, 1, ts))
	require.NoError(t, db.CreateItem(ctx, f.item2ID, "hammer", f.garageID, 2, ts))
	require.NoError(t, db.CreateItem(ctx, f.item3ID, "sandpaper", f.shelfAID, 3, ts))
	require.NoError(t, db.CreateItem(ctx, f.item4ID, "chisel", f.workbenchID, 4, ts))
	require.NoError(t, db.CreateItem(ctx, f.item5ID, "screwdriver", f.missingID, 5, ts))

	t.Cleanup(func() { db.Close() })

	return f
}

// newTestContext returns a context with the given config stored under config.ConfigKey.
func newTestContext(t *testing.T, cfg *config.Config) context.Context {
	t.Helper()

	return context.WithValue(t.Context(), config.ConfigKey, cfg)
}

// ---- buildLocationNodeFlat tests ----

func TestBuildLocationNodeFlat_ItemsPopulated(t *testing.T) {
	f := setupListTest(t)

	garage, err := f.db.GetLocation(f.ctx, f.garageID)
	require.NoError(t, err)

	node, err := buildLocationNodeFlat(f.ctx, f.db, garage)
	require.NoError(t, err)
	require.NotNil(t, node)

	assert.Len(t, node.Items, 2)
	assert.Equal(t, "drill", node.Items[0].DisplayName)
	assert.Equal(t, "hammer", node.Items[1].DisplayName)
}

func TestBuildLocationNodeFlat_ChildrenAreHintOnly(t *testing.T) {
	f := setupListTest(t)

	garage, err := f.db.GetLocation(f.ctx, f.garageID)
	require.NoError(t, err)

	node, err := buildLocationNodeFlat(f.ctx, f.db, garage)
	require.NoError(t, err)

	// Two children: ShelfA and Workbench
	assert.Len(t, node.Children, 2)

	for _, child := range node.Children {
		// Hint nodes have nil Items and nil Children slices.
		assert.Nil(t, child.Items, "flat child should not have Items populated")
		assert.Nil(t, child.Children, "flat child should not have Children populated")
		// But should have count metadata.
		assert.GreaterOrEqual(t, child.ChildItemCount, 0)
		assert.GreaterOrEqual(t, child.ChildLocationCount, 0)
	}
}

func TestBuildLocationNodeFlat_ChildItemAndLocationCounts(t *testing.T) {
	f := setupListTest(t)

	garage, err := f.db.GetLocation(f.ctx, f.garageID)
	require.NoError(t, err)

	node, err := buildLocationNodeFlat(f.ctx, f.db, garage)
	require.NoError(t, err)

	// Find ShelfA hint: 1 item, 0 locations.
	var shelfHint, workbenchHint *LocationNode
	for _, child := range node.Children {
		if child.Location.LocationID == f.shelfAID {
			shelfHint = child
		}
		if child.Location.LocationID == f.workbenchID {
			workbenchHint = child
		}
	}

	require.NotNil(t, shelfHint, "ShelfA hint missing")
	assert.Equal(t, 1, shelfHint.ChildItemCount)
	assert.Equal(t, 0, shelfHint.ChildLocationCount)

	require.NotNil(t, workbenchHint, "Workbench hint missing")
	assert.Equal(t, 1, workbenchHint.ChildItemCount)
	assert.Equal(t, 1, workbenchHint.ChildLocationCount)
}

func TestBuildLocationNodeFlat_EmptyLocation(t *testing.T) {
	f := setupListTest(t)

	office, err := f.db.GetLocation(f.ctx, f.officeID)
	require.NoError(t, err)

	node, err := buildLocationNodeFlat(f.ctx, f.db, office)
	require.NoError(t, err)

	assert.Empty(t, node.Items)
	assert.Empty(t, node.Children)
}

// ---- buildLocationNodeRecursive tests ----

func TestBuildLocationNodeRecursive_FullTree(t *testing.T) {
	f := setupListTest(t)

	garage, err := f.db.GetLocation(f.ctx, f.garageID)
	require.NoError(t, err)

	node, err := buildLocationNodeRecursive(f.ctx, f.db, garage)
	require.NoError(t, err)

	// Garage: 2 items, 2 children
	assert.Len(t, node.Items, 2)
	assert.Len(t, node.Children, 2)

	// Find Workbench child
	var workbench *LocationNode
	for _, child := range node.Children {
		if child.Location.LocationID == f.workbenchID {
			workbench = child
		}
	}
	require.NotNil(t, workbench)

	// Workbench: 1 item, 1 child (Drawer1)
	assert.Len(t, workbench.Items, 1)
	assert.Len(t, workbench.Children, 1)

	// Drawer1: 0 items, 0 children
	drawer1 := workbench.Children[0]
	assert.Empty(t, drawer1.Items)
	assert.Empty(t, drawer1.Children)
}

func TestBuildLocationNodeRecursive_EmptyLocation(t *testing.T) {
	f := setupListTest(t)

	office, err := f.db.GetLocation(f.ctx, f.officeID)
	require.NoError(t, err)

	node, err := buildLocationNodeRecursive(f.ctx, f.db, office)
	require.NoError(t, err)

	// Zero items, zero children.
	assert.Empty(t, node.Items)
	assert.Empty(t, node.Children)
}

// ---- buildNodes tests ----

func TestBuildNodes_NoArgs_UsesRoots(t *testing.T) {
	f := setupListTest(t)

	nodes, err := buildNodes(f.ctx, f.db, nil, false)
	require.NoError(t, err)
	// Should include Garage, Office, Missing (all roots).
	assert.GreaterOrEqual(t, len(nodes), 3)
	for _, n := range nodes {
		assert.False(t, n.NotFound)
	}
}

func TestBuildNodes_UnknownArg_NotFoundNode(t *testing.T) {
	f := setupListTest(t)

	nodes, err := buildNodes(f.ctx, f.db, []string{"DoesNotExist-xyz"}, false)
	require.NoError(t, err) // not-found does not cause an error
	require.Len(t, nodes, 1)
	assert.True(t, nodes[0].NotFound)
	assert.Equal(t, "DoesNotExist-xyz", nodes[0].InputArg)
}

func TestBuildNodes_MixedArgs_SomeFound(t *testing.T) {
	f := setupListTest(t)

	garage, err := f.db.GetLocation(f.ctx, f.garageID)
	require.NoError(t, err)

	nodes, err := buildNodes(f.ctx, f.db, []string{garage.CanonicalName, "NoSuchPlace"}, false)
	require.NoError(t, err)
	require.Len(t, nodes, 2)

	assert.False(t, nodes[0].NotFound)
	assert.True(t, nodes[1].NotFound)
	assert.Equal(t, "NoSuchPlace", nodes[1].InputArg)
}

func TestBuildNodes_Recurse_FullTree(t *testing.T) {
	f := setupListTest(t)

	garage, err := f.db.GetLocation(f.ctx, f.garageID)
	require.NoError(t, err)

	nodes, err := buildNodes(f.ctx, f.db, []string{garage.CanonicalName}, true)
	require.NoError(t, err)
	require.Len(t, nodes, 1)

	garageNode := nodes[0]
	assert.Len(t, garageNode.Children, 2)
	// Children should have their own Items populated (recursive mode).
	for _, child := range garageNode.Children {
		assert.NotNil(t, child.Items)
	}
}

// ---- JSON output tests ----

func TestToJSON_NotFoundNode(t *testing.T) {
	nodes := []*LocationNode{
		{NotFound: true, InputArg: "ghost"},
	}
	out := toJSON(nodes)
	require.Len(t, out.Locations, 1)
	assert.True(t, out.Locations[0].NotFound)
	assert.Equal(t, "ghost", out.Locations[0].DisplayName)
}

func TestToJSON_SingleLocation(t *testing.T) {
	f := setupListTest(t)

	garage, err := f.db.GetLocation(f.ctx, f.garageID)
	require.NoError(t, err)

	node, err := buildLocationNodeFlat(f.ctx, f.db, garage)
	require.NoError(t, err)

	out := toJSON([]*LocationNode{node})
	require.Len(t, out.Locations, 1)

	loc := out.Locations[0]
	assert.Equal(t, f.garageID, loc.LocationID)
	assert.Equal(t, 2, loc.ItemCount)
	assert.Equal(t, 2, loc.LocationCount)
	assert.Len(t, loc.Items, 2)
	assert.Len(t, loc.Children, 2)
}

func TestToJSON_JSONRoundtrip(t *testing.T) {
	f := setupListTest(t)

	garage, err := f.db.GetLocation(f.ctx, f.garageID)
	require.NoError(t, err)

	node, err := buildLocationNodeRecursive(f.ctx, f.db, garage)
	require.NoError(t, err)

	out := toJSON([]*LocationNode{node})

	data, marshalErr := json.Marshal(out)
	require.NoError(t, marshalErr)

	var back OutputJSON
	require.NoError(t, json.Unmarshal(data, &back))

	assert.Equal(t, out.Locations[0].LocationID, back.Locations[0].LocationID)
	assert.Equal(t, out.Locations[0].ItemCount, back.Locations[0].ItemCount)
}

// ---- renderTree tests ----

func TestRenderTree_EmptyNodes(t *testing.T) {
	var buf bytes.Buffer
	renderTree(&buf, nil)
	assert.Empty(t, buf.String())
}

func TestRenderTree_NotFoundNode(t *testing.T) {
	var buf bytes.Buffer
	renderTree(&buf, []*LocationNode{{NotFound: true, InputArg: "old-shelf"}})
	assert.Contains(t, buf.String(), "old-shelf [not found]")
}

func TestRenderTree_SingleLocationNoItems(t *testing.T) {
	loc := &database.Location{DisplayName: "Office", LocationID: "id-office"}
	node := &LocationNode{
		Location: loc,
		Items:    []*database.Item{},
		Children: []*LocationNode{},
	}

	var buf bytes.Buffer
	renderTree(&buf, []*LocationNode{node})

	output := stripANSI(buf.String())
	assert.Contains(t, output, "Office (0 items, 0 locations)")
}

func TestRenderTree_ItemsShownWithConnectors(t *testing.T) {
	loc := &database.Location{DisplayName: "Garage", LocationID: "id-garage"}
	node := &LocationNode{
		Location: loc,
		Items: []*database.Item{
			{DisplayName: "drill"},
			{DisplayName: "hammer"},
		},
		Children: []*LocationNode{},
	}

	var buf bytes.Buffer
	renderTree(&buf, []*LocationNode{node})

	output := buf.String()
	assert.Contains(t, output, "drill")
	assert.Contains(t, output, "hammer")
	// treeprint uses box-drawing characters for connectors
	assert.Contains(t, output, "└──")
}

func TestRenderTree_TemporaryUseItemHasStar(t *testing.T) {
	loc := &database.Location{DisplayName: "Garage", LocationID: "id-garage"}
	node := &LocationNode{
		Location: loc,
		Items: []*database.Item{
			{DisplayName: "drill", InTemporaryUse: true},
		},
		Children: []*LocationNode{},
	}

	var buf bytes.Buffer
	renderTree(&buf, []*LocationNode{node})

	output := buf.String()
	assert.Contains(t, output, "drill *")
}

func TestRenderTree_FlatChildHints(t *testing.T) {
	loc := &database.Location{DisplayName: "Garage", LocationID: "id-garage"}
	shelfLoc := &database.Location{DisplayName: "Shelf A", LocationID: "id-shelf"}
	node := &LocationNode{
		Location: loc,
		Items:    []*database.Item{},
		Children: []*LocationNode{
			{
				Location:           shelfLoc,
				ChildItemCount:     1,
				ChildLocationCount: 3,
			},
		},
	}

	var buf bytes.Buffer
	renderTree(&buf, []*LocationNode{node})

	output := buf.String()
	assert.Contains(t, output, "[Shelf A]")
	assert.Contains(t, output, "1 item")
	assert.Contains(t, output, "3 locations")
}

func TestRenderTree_MultipleRootsSeparatedByBlankLines(t *testing.T) {
	loc1 := &database.Location{DisplayName: "Garage", LocationID: "id-1"}
	loc2 := &database.Location{DisplayName: "Office", LocationID: "id-2"}

	nodes := []*LocationNode{
		{Location: loc1, Items: []*database.Item{}, Children: []*LocationNode{}},
		{Location: loc2, Items: []*database.Item{}, Children: []*LocationNode{}},
	}

	var buf bytes.Buffer
	renderTree(&buf, nodes)

	output := buf.String()
	// Blank line between the two trees
	assert.Contains(t, output, "\n\n")
	assert.Contains(t, output, "Garage")
	assert.Contains(t, output, "Office")
}

func TestRenderTree_PluralSingular(t *testing.T) {
	loc := &database.Location{DisplayName: "Test", LocationID: "id-test"}
	child := &database.Location{DisplayName: "Sub", LocationID: "id-sub"}
	node := &LocationNode{
		Location: loc,
		Items:    []*database.Item{{DisplayName: "widget"}},
		Children: []*LocationNode{
			{
				Location:           child,
				ChildItemCount:     1,
				ChildLocationCount: 1,
			},
		},
	}

	var buf bytes.Buffer
	renderTree(&buf, []*LocationNode{node})

	output := buf.String()
	// Header: "1 item" (not "1 items")
	assert.Contains(t, output, "1 item,")
	// Sub-location hint: "1 item, 1 location"
	assert.Contains(t, output, "1 location")
	// Ensure "items" plural is NOT used for count of 1
	lines := strings.SplitSeq(output, "\n")
	for line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "Test") {
			assert.NotContains(t, line, "1 items")
		}
	}
}

func TestRenderTree_RecursiveTree(t *testing.T) {
	f := setupListTest(t)

	garage, err := f.db.GetLocation(f.ctx, f.garageID)
	require.NoError(t, err)

	node, err := buildLocationNodeRecursive(f.ctx, f.db, garage)
	require.NoError(t, err)

	var buf bytes.Buffer
	renderTree(&buf, []*LocationNode{node})

	output := buf.String()
	assert.Contains(t, output, "drill")
	assert.Contains(t, output, "hammer")
	assert.Contains(t, output, "sandpaper")
	assert.Contains(t, output, "chisel")
}

// ---- locationHeader unit tests ----

func TestLocationHeader_NoItemsNoLocations(t *testing.T) {
	result := locationHeader("Office", 0, 0)
	assert.Equal(t, "Office (0 items, 0 locations)", stripANSI(result))
}

func TestLocationHeader_SingleItemNoLocations(t *testing.T) {
	result := locationHeader("Shelf", 1, 0)
	assert.Equal(t, "Shelf (1 item, 0 locations)", stripANSI(result))
}

func TestLocationHeader_MultipleItemsNoLocations(t *testing.T) {
	result := locationHeader("Garage", 2, 0)
	assert.Equal(t, "Garage (2 items, 0 locations)", stripANSI(result))
}

func TestLocationHeader_NoItemsSingleLocation(t *testing.T) {
	result := locationHeader("Cabinet", 0, 1)
	assert.Equal(t, "Cabinet (0 items, 1 location)", stripANSI(result))
}

func TestLocationHeader_NoItemsMultipleLocations(t *testing.T) {
	result := locationHeader("Workshop", 0, 3)
	assert.Equal(t, "Workshop (0 items, 3 locations)", stripANSI(result))
}

func TestLocationHeader_MultipleItemsAndLocations(t *testing.T) {
	result := locationHeader("Basement", 5, 2)
	assert.Equal(t, "Basement (5 items, 2 locations)", stripANSI(result))
}

// ---- runListCore integration tests ----

func TestRunList_NoArgs_ShowsAllRootLocations(t *testing.T) {
	f := setupListTest(t)

	// Fetch expected location names before closing database
	garage, _ := f.db.GetLocation(f.ctx, f.garageID)
	office, _ := f.db.GetLocation(f.ctx, f.officeID)
	missing, _ := f.db.GetLocation(f.ctx, f.missingID)

	require.NotNil(t, garage)
	require.NotNil(t, office)
	require.NotNil(t, missing)

	garageDisplayName := garage.DisplayName
	officeDisplayName := office.DisplayName
	missingDisplayName := missing.DisplayName

	testCfg := &config.Config{
		Output: config.OutputConfig{DefaultFormat: "text"},
	}
	ctx := newTestContext(t, testCfg)

	var buf bytes.Buffer
	cmd := NewListCmd()
	cmd.SetOut(&buf)
	cmd.SetContext(ctx)

	err := runListCore(cmd, []string{}, f.db)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, garageDisplayName)
	assert.Contains(t, output, officeDisplayName)
	assert.Contains(t, output, missingDisplayName)
}

func TestRunList_SingleArgFound_ShowsLocationItems(t *testing.T) {
	f := setupListTest(t)

	garage, _ := f.db.GetLocation(f.ctx, f.garageID)
	require.NotNil(t, garage)

	garageCanonicalName := garage.CanonicalName
	garageDisplayName := garage.DisplayName

	testCfg := &config.Config{
		Output: config.OutputConfig{DefaultFormat: "text"},
	}
	ctx := newTestContext(t, testCfg)

	var buf bytes.Buffer
	cmd := NewListCmd()
	cmd.SetOut(&buf)
	cmd.SetContext(ctx)

	err := runListCore(cmd, []string{garageCanonicalName}, f.db)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, garageDisplayName)
	assert.Contains(t, output, "drill")
	assert.Contains(t, output, "hammer")
}

func TestRunList_SingleArgNotFound_RendersNotFound(t *testing.T) {
	f := setupListTest(t)

	testCfg := &config.Config{
		Output: config.OutputConfig{DefaultFormat: "text"},
	}
	ctx := newTestContext(t, testCfg)

	var buf bytes.Buffer
	cmd := NewListCmd()
	cmd.SetOut(&buf)
	cmd.SetContext(ctx)

	err := runListCore(cmd, []string{"does_not_exist"}, f.db)
	require.NoError(t, err) // no error on not-found

	output := buf.String()
	assert.Contains(t, output, "does_not_exist [not found]")
}

func TestRunList_MixedArgsBothRender(t *testing.T) {
	f := setupListTest(t)

	garage, _ := f.db.GetLocation(f.ctx, f.garageID)
	require.NotNil(t, garage)

	garageCanonicalName := garage.CanonicalName
	garageDisplayName := garage.DisplayName

	testCfg := &config.Config{
		Output: config.OutputConfig{DefaultFormat: "text"},
	}
	ctx := newTestContext(t, testCfg)

	var buf bytes.Buffer
	cmd := NewListCmd()
	cmd.SetOut(&buf)
	cmd.SetContext(ctx)

	err := runListCore(cmd, []string{garageCanonicalName, "ghost_location"}, f.db)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, garageDisplayName)
	assert.Contains(t, output, "drill")
	assert.Contains(t, output, "ghost_location [not found]")
}

func TestRunList_RecurseFlag_IncludesSublocationsRecursively(t *testing.T) {
	f := setupListTest(t)

	garage, _ := f.db.GetLocation(f.ctx, f.garageID)
	require.NotNil(t, garage)

	garageDisplayName := garage.DisplayName

	testCfg := &config.Config{
		Output: config.OutputConfig{DefaultFormat: "text"},
	}
	ctx := newTestContext(t, testCfg)

	var buf bytes.Buffer
	cmd := NewListCmd()
	cmd.SetOut(&buf)
	// cmd.SetArgs([]string{"--recurse"})
	cmd.Flags().Set("recurse", "true")
	cmd.SetContext(ctx)

	err := runListCore(cmd, []string{}, f.db)
	require.NoError(t, err)

	output := buf.String()
	// In recursive mode, should show items from grandchild (Drawer1 is a child of Workbench)
	assert.Contains(t, output, garageDisplayName)
	assert.Contains(t, output, "drill")
	assert.Contains(t, output, "hammer")
	assert.Contains(t, output, "sandpaper") // from ShelfA
	assert.Contains(t, output, "chisel")    // from Workbench
}

func TestRunList_JSONFlag_OutputsValidJSON(t *testing.T) {
	f := setupListTest(t)

	garage, _ := f.db.GetLocation(f.ctx, f.garageID)
	require.NotNil(t, garage)

	garageID := garage.LocationID
	garageCanonicalName := garage.CanonicalName

	testCfg := &config.Config{
		Output: config.OutputConfig{DefaultFormat: "json"},
	}
	ctx := newTestContext(t, testCfg)

	var buf bytes.Buffer
	cmd := NewListCmd()
	cmd.SetOut(&buf)
	cmd.SetContext(ctx)

	err := runListCore(cmd, []string{garageCanonicalName}, f.db)
	require.NoError(t, err)

	// Verify output is valid JSON
	var result OutputJSON
	err = json.Unmarshal(buf.Bytes(), &result)
	require.NoError(t, err)

	require.Len(t, result.Locations, 1)
	assert.Equal(t, garageID, result.Locations[0].LocationID)
	assert.Equal(t, 2, result.Locations[0].ItemCount)
	assert.Equal(t, 2, result.Locations[0].LocationCount)
}

func TestRunList_JSONWithNotFound_IncludesNotFoundMarker(t *testing.T) {
	f := setupListTest(t)

	garage, _ := f.db.GetLocation(f.ctx, f.garageID)
	require.NotNil(t, garage)

	garageCanonicalName := garage.CanonicalName

	testCfg := &config.Config{
		Output: config.OutputConfig{DefaultFormat: "json"},
	}
	ctx := newTestContext(t, testCfg)

	var buf bytes.Buffer
	cmd := NewListCmd()
	cmd.SetOut(&buf)
	cmd.SetContext(ctx)

	err := runListCore(cmd, []string{garageCanonicalName, "nonexistent_location"}, f.db)
	require.NoError(t, err)

	var result OutputJSON
	err = json.Unmarshal(buf.Bytes(), &result)
	require.NoError(t, err)

	// Should have 2 locations: one found, one not found
	require.Len(t, result.Locations, 2)
	assert.False(t, result.Locations[0].NotFound)
	assert.True(t, result.Locations[1].NotFound)
	assert.Equal(t, "nonexistent_location", result.Locations[1].DisplayName)
}
