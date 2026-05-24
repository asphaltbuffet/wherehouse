package entitypath_test

import (
	"fmt"

	"github.com/asphaltbuffet/wherehouse/internal/entitypath"
)

func ExampleParse() {
	p, err := entitypath.Parse(":Garage:Shelf A")
	if err != nil {
		panic(err)
	}
	fmt.Println(p)
	fmt.Println(p.IsAbs())
	fmt.Println(p.Depth())
	// Output:
	// :Garage:Shelf A
	// true
	// 2
}

func ExamplePath_Join() {
	p := entitypath.MustParse(":Garage:Shelf A")
	child, err := p.Join("Bin 3")
	if err != nil {
		panic(err)
	}
	fmt.Println(child)
	fmt.Println(p.IsAncestor(child))
	// Output:
	// :Garage:Shelf A:Bin 3
	// true
}

func ExamplePath_Rel() {
	base := entitypath.MustParse(":Garage")
	child := entitypath.MustParse(":Garage:Shelf A:Bin 3")
	rel, err := child.Rel(base)
	if err != nil {
		panic(err)
	}
	fmt.Println(rel)
	// Output:
	// Shelf A:Bin 3
}

func ExamplePath_Walk() {
	p := entitypath.MustParse(":Garage:Shelf A:Bin 3")
	p.Walk(func(a entitypath.Path) bool {
		fmt.Println(a)
		return true
	})
	// Output:
	// :Garage:Shelf A
	// :Garage
	// :
}

func ExamplePath_Clean() {
	p := entitypath.Path("Foo::Bar:")
	fmt.Println(p.Clean())
	// Output:
	// Foo:Bar
}
