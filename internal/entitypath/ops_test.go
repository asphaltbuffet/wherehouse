package entitypath_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/entitypath"
)

// --- Join ---

func TestPath_Join_Success(t *testing.T) {
	parent := entitypath.MustParse(":Garage:Shelf A")
	got, err := parent.Join("Bin 3", "Resistors")

	require.NoError(t, err)

	assert.Equal(t, entitypath.Path(":Garage:Shelf A:Bin 3:Resistors"), got)
	assert.Equal(t, 4, got.Depth())
	assert.True(t, got.IsAbs())
	assert.True(t, parent.IsAncestor(got))
}

func TestPath_Join_NoSegments(t *testing.T) {
	p := entitypath.MustParse(":Garage")
	got, err := p.Join()

	require.NoError(t, err)

	assert.Equal(t, p, got)
}

func TestPath_Join_OnEmpty(t *testing.T) {
	got, err := entitypath.Path("").Join("Garage")
	require.NoError(t, err)

	assert.Equal(t, entitypath.Path("Garage"), got)
}

func TestPath_Join_OnRoot(t *testing.T) {
	got, err := entitypath.Root.Join("Garage")
	require.NoError(t, err)

	assert.Equal(t, entitypath.Path(":Garage"), got)
}

func TestPath_Join_RejectsBadSegment(t *testing.T) {
	parent := entitypath.MustParse(":Garage")

	_, err := parent.Join("Shelf:A")
	require.Error(t, err)
	require.ErrorIs(t, err, entitypath.ErrSegmentContainsSeparator)
	require.ErrorIs(t, err, entitypath.ErrInvalidPath)

	_, err = parent.Join("")
	require.Error(t, err)
	require.ErrorIs(t, err, entitypath.ErrEmptySegment)

	_, err = parent.Join("  spaced  ")
	require.Error(t, err)
	require.ErrorIs(t, err, entitypath.ErrInvalidSegment)
}

// --- Append ---

func TestPath_Append(t *testing.T) {
	p := entitypath.MustParse(":Garage")
	got, err := p.Append("Shelf A")
	require.NoError(t, err)

	assert.Equal(t, entitypath.Path(":Garage:Shelf A"), got)
}

func TestPath_Append_RejectsEmpty(t *testing.T) {
	p := entitypath.MustParse(":Garage")
	_, err := p.Append("")

	assert.ErrorIs(t, err, entitypath.ErrEmptySegment)
}

// --- Clean ---

func TestClean_Idempotent(t *testing.T) {
	inputs := []string{
		"",
		":",
		"Foo",
		":Foo",
		"Foo:Bar",
		":Foo:Bar",
		"Foo::Bar",
		"Foo:",
		"::Foo",
		":Foo:Bar:",
	}

	for _, s := range inputs {
		t.Run(s, func(t *testing.T) {
			p := entitypath.Path(s)
			once := p.Clean()
			twice := once.Clean()

			assert.Equal(t, once, twice, "Clean not idempotent")
		})
	}
}

func TestClean_ExpectedOutputs(t *testing.T) {
	tests := []struct {
		input string
		want  entitypath.Path
	}{
		{"", entitypath.Path("")},
		{":", entitypath.Root},
		{"Foo", entitypath.Path("Foo")},
		{":Foo", entitypath.Path(":Foo")},
		{"Foo:", entitypath.Path("Foo")},
		{"Foo::Bar", entitypath.Path("Foo:Bar")},
		{"::Foo", entitypath.Path(":Foo")},
		{":Foo:Bar:", entitypath.Path(":Foo:Bar")},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := entitypath.Path(tt.input).Clean()

			assert.Equal(t, tt.want, got)
		})
	}
}

// --- Rel ---

func TestPath_Rel_EqualPaths(t *testing.T) {
	p := entitypath.MustParse(":Garage:Shelf A")
	got, err := p.Rel(p)
	require.NoError(t, err)

	assert.Equal(t, entitypath.Path(""), got)
}

func TestPath_Rel_Ancestor(t *testing.T) {
	base := entitypath.MustParse(":Garage")
	child := entitypath.MustParse(":Garage:Shelf A:Bin 3")

	got, err := child.Rel(base)
	require.NoError(t, err)

	assert.Equal(t, entitypath.Path("Shelf A:Bin 3"), got)
}

func TestPath_Rel_RootBase(t *testing.T) {
	child := entitypath.MustParse(":Garage:Shelf A")
	got, err := child.Rel(entitypath.Root)
	require.NoError(t, err)

	assert.Equal(t, entitypath.Path("Garage:Shelf A"), got)
}

func TestPath_Rel_NotDescendant(t *testing.T) {
	base := entitypath.MustParse(":Garage:Shelf A")
	other := entitypath.MustParse(":Kitchen:Drawer 1")
	_, err := other.Rel(base)
	require.Error(t, err)

	assert.ErrorIs(t, err, entitypath.ErrNotDescendant)
}

func TestPath_Rel_MismatchedAbsRel(t *testing.T) {
	rel := entitypath.MustParse("Garage:Shelf A:Bin 1")
	abs := entitypath.MustParse(":Garage:Shelf A")
	_, err := rel.Rel(abs)

	assert.ErrorIs(t, err, entitypath.ErrNotDescendant)
}

// --- IsAncestor ---

func TestPath_IsAncestor(t *testing.T) {
	tests := []struct {
		name  string
		p     string
		other string
		want  bool
	}{
		{"self is not an ancestor", ":Garage", ":Garage", false},
		{"proper ancestor", ":Garage", ":Garage:Shelf A", true},
		{"deeper ancestor", ":Garage", ":Garage:Shelf A:Bin 3", true},
		{"not ancestor (sibling)", ":Garage", ":Kitchen", false},
		{"segment-boundary: prefix match must land on boundary", ":Foo:Ba", ":Foo:Bar", false},
		{"relative paths", "Foo", "Foo:Bar", true},
		{"substring", "Foo", "FooBar", false},
		{"mismatched abs/rel", ":Garage", "Garage:Shelf A", false},
		{"root is ancestor of everything absolute", ":", ":Garage", true},
		{"root + nested abs", ":", ":Garage:Shelf A", true},
		{"root is not ancestor of relative", ":", "Garage", false},
	}
	for _, tt := range tests {
		t.Run(tt.p+"→"+tt.other, func(t *testing.T) {
			p := entitypath.MustParse(tt.p)
			other := entitypath.MustParse(tt.other)

			assert.Equal(t, tt.want, p.IsAncestor(other))
		})
	}
}

// --- HasPrefix ---

func TestPath_HasPrefix(t *testing.T) {
	p := entitypath.MustParse(":Garage:Shelf A:Bin 3")

	assert.True(t, p.HasPrefix(p), "self")
	assert.True(t, p.HasPrefix(entitypath.MustParse(":Garage")), "ancestor")
	assert.True(t, p.HasPrefix(entitypath.Root), "root")

	assert.False(t, p.HasPrefix(entitypath.MustParse(":Kitchen")), "unrelated")
	assert.False(t, p.HasPrefix(entitypath.MustParse(":Garage:Shelf")), "mid-segment")
}

// --- Ancestors ---

func TestPath_Ancestors_Order(t *testing.T) {
	p := entitypath.MustParse(":Garage:Shelf A:Bin 3")
	want := []entitypath.Path{
		entitypath.Root,
		entitypath.MustParse(":Garage"),
		entitypath.MustParse(":Garage:Shelf A"),
	}

	got := p.Ancestors()

	assert.Equal(t, want, got)
}

func TestPath_Ancestors_SingleSegment(t *testing.T) {
	p := entitypath.MustParse(":Garage")
	got := p.Ancestors()

	assert.Equal(t, []entitypath.Path{entitypath.Root}, got)
}

func TestPath_Ancestors_Empty(t *testing.T) {
	assert.Equal(t, []entitypath.Path{}, entitypath.Path("").Ancestors())

	assert.Equal(t, []entitypath.Path{}, entitypath.Root.Ancestors())
}

func TestPath_Ancestors_Relative(t *testing.T) {
	p := entitypath.MustParse("Garage:Shelf A:Bin 3")
	want := []entitypath.Path{
		entitypath.MustParse("Garage"),
		entitypath.MustParse("Garage:Shelf A"),
	}

	got := p.Ancestors()

	assert.Equal(t, want, got)
}

// --- Walk ---

func TestPath_Walk_Order(t *testing.T) {
	p := entitypath.MustParse(":Garage:Shelf A:Bin 3")
	// nearest-first
	want := []entitypath.Path{
		entitypath.MustParse(":Garage:Shelf A"),
		entitypath.MustParse(":Garage"),
		entitypath.Root,
	}

	var got []entitypath.Path
	p.Walk(func(a entitypath.Path) bool {
		got = append(got, a)
		return true
	})

	assert.Equal(t, want, got)
}

func TestPath_Walk_EarlyBreak(t *testing.T) {
	p := entitypath.MustParse(":Garage:Shelf A:Bin 3")
	var got []entitypath.Path
	p.Walk(func(a entitypath.Path) bool {
		got = append(got, a)
		return false // stop after first
	})

	assert.Len(t, got, 1)
	assert.Equal(t, entitypath.MustParse(":Garage:Shelf A"), got[0])
}

func TestPath_Walk_Empty(t *testing.T) {
	var got []entitypath.Path
	entitypath.Path("").Walk(func(a entitypath.Path) bool {
		got = append(got, a)
		return true
	})
	assert.Empty(t, got)

	entitypath.Root.Walk(func(a entitypath.Path) bool {
		got = append(got, a)
		return true
	})
	assert.Empty(t, got)
}
