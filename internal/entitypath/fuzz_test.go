package entitypath_test

import (
	"testing"

	"github.com/asphaltbuffet/wherehouse/internal/entitypath"
)

func FuzzParseRoundtrip(f *testing.F) {
	seeds := []string{
		"",
		":",
		"Garage",
		":Garage",
		"Garage:Shelf A",
		":Garage:Shelf A:Bin 3",
		":🔧wrench",
		":Big Toolbox",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, s string) {
		p, err := entitypath.Parse(s)
		if err != nil {
			return // invalid input: not a roundtrip concern
		}

		p2, err := entitypath.Parse(p.String())
		if err != nil {
			t.Fatalf("Parse(%q).String() = %q failed to re-parse: %v", s, p.String(), err)
		}

		if p != p2 {
			t.Fatalf("roundtrip mismatch: Parse(%q) = %q, re-parse = %q", s, p, p2)
		}
	})
}

func FuzzCleanIdempotent(f *testing.F) {
	seeds := []string{
		"",
		":",
		"Foo",
		":Foo",
		"Foo:Bar",
		"Foo::Bar",
		"Foo:",
		"::Foo",
		":Foo:Bar:",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, s string) {
		p := entitypath.Path(s)
		once := p.Clean()
		twice := once.Clean()
		if once != twice {
			t.Fatalf("Clean not idempotent for %q: first=%q second=%q", s, once, twice)
		}
	})
}

func FuzzJoinSegments(f *testing.F) {
	f.Add("Garage", "Shelf A", "Bin 3")
	f.Add("foo", "bar", "")
	f.Add("", "x", "y")
	f.Add("x:y", "z", "")

	f.Fuzz(func(t *testing.T, a, b, c string) {
		segs := []string{a, b, c}
		result, err := entitypath.New(segs...)
		if err != nil {
			return
		}
		got := result.Segments()
		if len(got) != len(segs) {
			t.Fatalf("New(%v).Segments() = %v, want %d segments", segs, got, len(segs))
		}
		for i, seg := range segs {
			if got[i] != seg {
				t.Fatalf("New(%v).Segments()[%d] = %q, want %q", segs, i, got[i], seg)
			}
		}
	})
}
