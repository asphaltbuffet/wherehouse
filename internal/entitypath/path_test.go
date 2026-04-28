package entitypath_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/entitypath"
)

func TestValidateSegment(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr error
	}{
		{"empty string", "", entitypath.ErrEmptySegment},
		{"contains separator colon", "foo:bar", entitypath.ErrSegmentContainsSeparator},
		{"contains double colon", "foo::bar", entitypath.ErrSegmentContainsSeparator},
		{"leading whitespace", " foo", entitypath.ErrInvalidSegment},
		{"trailing whitespace", "foo ", entitypath.ErrInvalidSegment},
		{"control char tab", "foo\tbar", entitypath.ErrInvalidSegment},
		{"control char newline", "foo\nbar", entitypath.ErrInvalidSegment},
		{"valid plain", "foo", nil},
		{"valid with internal space", "Big Toolbox", nil},
		{"valid emoji", "🔧wrench", nil},
		{"valid non-ascii", "étagère", nil},
		{"valid unicode", "工具", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := entitypath.ValidateSegment(tt.input)

			if tt.wantErr == nil {
				assert.NoError(t, got)
			} else {
				assert.ErrorIs(t, got, tt.wantErr)
			}
		})
	}
}

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    entitypath.Path
		wantErr error
	}{
		{"empty string is valid", "", entitypath.Path(""), nil},
		{"root is valid", ":", entitypath.Root, nil},
		{"relative single segment", "Garage", entitypath.Path("Garage"), nil},
		{"absolute single segment", ":Garage", entitypath.Path(":Garage"), nil},
		{"relative multi-segment", "Garage:Shelf A", entitypath.Path("Garage:Shelf A"), nil},
		{"absolute multi-segment", ":Garage:Shelf A", entitypath.Path(":Garage:Shelf A"), nil},
		{"segment with emoji", ":🔧wrench", entitypath.Path(":🔧wrench"), nil},
		{"segment with internal space", ":Big Toolbox", entitypath.Path(":Big Toolbox"), nil},
		{"trailing separator", "Foo:", entitypath.Path(""), entitypath.ErrInvalidPath},
		{"adjacent separators", "Foo::Bar", entitypath.Path(""), entitypath.ErrInvalidPath},
		{"absolute two-level with colons as separators", ":foo:bar", entitypath.Path(":foo:bar"), nil},
		{"leading space in segment", ": foo", entitypath.Path(""), entitypath.ErrInvalidPath},
		{"trailing space in segment", ":foo ", entitypath.Path(""), entitypath.ErrInvalidPath},
		{"control char in segment", ":foo\nbar", entitypath.Path(""), entitypath.ErrInvalidPath},
		{"absolute multi-level", ":foo:bar:baz:qux", entitypath.Path(":foo:bar:baz:qux"), nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := entitypath.Parse(tt.input)

			assert.Equal(t, tt.want, got)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestParse_EmptySegmentErrors(t *testing.T) {
	// Trailing separator emits ErrEmptySegment wrapped under ErrInvalidPath.
	_, err := entitypath.Parse("Foo:")
	require.Error(t, err)
	require.ErrorIs(t, err, entitypath.ErrInvalidPath)
	require.ErrorIs(t, err, entitypath.ErrEmptySegment)
}

func TestParse_ColonInSegmentErrors(t *testing.T) {
	// A colon within a non-leading position is parsed as a separator,
	// producing an empty or colon-containing segment — both invalid.
	_, err := entitypath.Parse("Foo::Bar")
	require.Error(t, err)
	require.ErrorIs(t, err, entitypath.ErrInvalidPath)
	require.ErrorIs(t, err, entitypath.ErrEmptySegment)
}

func TestNew(t *testing.T) {
	tests := []struct {
		name     string
		segments []string
		want     entitypath.Path
		wantErr  error
	}{
		{"no segments", []string{}, entitypath.Path(""), nil},
		{"single segment", []string{"Garage"}, entitypath.Path("Garage"), nil},
		{"multi segment", []string{"Garage", "Shelf A"}, entitypath.Path("Garage:Shelf A"), nil},
		{"empty segment", []string{"Garage", ""}, entitypath.Path(""), entitypath.ErrEmptySegment},
		{"colon in segment", []string{"foo:bar"}, entitypath.Path(""), entitypath.ErrSegmentContainsSeparator},
		{"leading whitespace in segment", []string{" foo"}, entitypath.Path(""), entitypath.ErrInvalidSegment},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := entitypath.New(tt.segments...)

			if tt.wantErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsAbs(t *testing.T) {
	tests := []struct {
		path          string
		boolAssertion assert.BoolAssertionFunc
	}{
		{"", assert.False},
		{":", assert.True},
		{":Garage", assert.True},
		{":Garage:Shelf A", assert.True},
		{"Garage", assert.False},
		{"Garage:Shelf A", assert.False},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			tt.boolAssertion(t, entitypath.MustParse(tt.path).IsAbs())
		})
	}
}

func TestIsRoot(t *testing.T) {
	assert.True(t, entitypath.Root.IsRoot())

	assert.False(t, entitypath.MustParse("").IsRoot())
	assert.False(t, entitypath.MustParse(":Garage").IsRoot())
}

func TestIsEmpty(t *testing.T) {
	assert.True(t, entitypath.Path("").IsEmpty())

	assert.False(t, entitypath.Root.IsEmpty())
	assert.False(t, entitypath.MustParse("Garage").IsEmpty())
}

func TestDepth(t *testing.T) {
	tests := []struct {
		path string
		want int
	}{
		{"", 0},
		{":", 0},
		{"Garage", 1},
		{":Garage", 1},
		{"Garage:Shelf A", 2},
		{":Garage:Shelf A:Bin 3", 3},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			assert.Equal(t, tt.want, entitypath.MustParse(tt.path).Depth())
		})
	}
}

func TestSegments(t *testing.T) {
	tests := []struct {
		path string
		want []string
	}{
		{"", []string{}},
		{":", []string{}},
		{"Garage", []string{"Garage"}},
		{":Garage", []string{"Garage"}},
		{"Garage:Shelf A", []string{"Garage", "Shelf A"}},
		{":Garage:Shelf A:Bin 3", []string{"Garage", "Shelf A", "Bin 3"}},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := entitypath.MustParse(tt.path).Segments()

			assert.Equal(t, tt.want, got)
		})
	}
}

func TestBase(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"", ""},
		{":", ""},
		{"Garage", "Garage"},
		{":Garage", "Garage"},
		{"Garage:Shelf A", "Shelf A"},
		{":Garage:Shelf A:Bin 3", "Bin 3"},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			assert.Equal(t, tt.want, entitypath.MustParse(tt.path).Base())
		})
	}
}

func TestDir(t *testing.T) {
	tests := []struct {
		path string
		want entitypath.Path
	}{
		{"", entitypath.Path("")},
		{":", entitypath.Root},
		{"Garage", entitypath.Path("")},
		{":Garage", entitypath.Root},
		{"Garage:Shelf A", entitypath.Path("Garage")},
		{":Garage:Shelf A", entitypath.Path(":Garage")},
		{":Garage:Shelf A:Bin 3", entitypath.Path(":Garage:Shelf A")},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			assert.Equal(t, tt.want, entitypath.MustParse(tt.path).Dir())
		})
	}
}

func TestDir_JoinBase_Roundtrip(t *testing.T) {
	// For non-root non-empty paths: Dir(p).Join(Base(p)) == p.
	paths := []string{
		"Garage",
		":Garage",
		"Garage:Shelf A",
		":Garage:Shelf A",
		":Garage:Shelf A:Bin 3",
	}
	for _, s := range paths {
		t.Run(s, func(t *testing.T) {
			p := entitypath.MustParse(s)
			got, err := p.Dir().Join(p.Base())

			require.NoError(t, err)
			assert.Equal(t, p, got)
		})
	}
}

func TestParse_Roundtrip(t *testing.T) {
	paths := []string{
		"",
		":",
		"Garage",
		":Garage",
		"Garage:Shelf A",
		":Garage:Shelf A:Bin 3:Resistors",
		":🔧wrench",
		":Big Toolbox",
	}
	for _, s := range paths {
		t.Run(s, func(t *testing.T) {
			p, err := entitypath.Parse(s)
			require.NoError(t, err)

			got, err := entitypath.Parse(p.String())

			require.NoError(t, err)
			assert.Equal(t, p, got)
		})
	}
}

func TestMustParse_Panics(t *testing.T) {
	assert.Panics(t, func() {
		entitypath.MustParse("bad:")
	})
}

func TestMustParse_ValidDoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		entitypath.MustParse(":Garage:Shelf A")
	})
}

func TestJSON_Roundtrip(t *testing.T) {
	paths := []entitypath.Path{
		entitypath.Path(""),
		entitypath.Root,
		entitypath.MustParse(":Garage:Shelf A"),
	}

	for _, p := range paths {
		t.Run(p.String(), func(t *testing.T) {
			b, err := p.MarshalJSON()
			require.NoError(t, err)

			var got entitypath.Path

			require.NoError(t, got.UnmarshalJSON(b))
			assert.Equal(t, p, got)
		})
	}
}

func TestJSON_UnmarshalRejectsInvalid(t *testing.T) {
	var p entitypath.Path
	err := p.UnmarshalJSON([]byte(`"bad:"`))
	require.Error(t, err)
	assert.ErrorIs(t, err, entitypath.ErrInvalidPath)
}
