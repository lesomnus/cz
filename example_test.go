package cz_test

import (
	"fmt"

	"github.com/lesomnus/cz"
)

func ExampleRender() {
	user := cz.Ident("user")
	expr := cz.And(
		cz.Ge(user.Select("age"), cz.Int(18)),
		cz.In(user.Select("country"), cz.List(cz.String("KR"), cz.String("US"))),
	)
	source, err := cz.Render(expr)
	if err != nil {
		panic(err)
	}
	fmt.Println(source)
	// Output: user.age >= 18 && user.country in ["KR", "US"]
}

func Example_watch() {
	event := cz.Ident("event")
	value := event.Select("value")
	expr := cz.And(
		cz.Has(value),
		cz.Eq(cz.Type(value), cz.Ident("app.Manifest")),
		cz.Eq(event.Select("kind"), cz.Ident("payday.CHANGE_KIND_CREATED")),
		cz.Eq(value.Select("artifact_type"), cz.String("application/vnd.dev.sigstore.bundle.v0.3+json")),
	)
	filter, err := cz.Render(expr)
	if err != nil {
		panic(err)
	}
	fmt.Println(filter)
	// Output: has(event.value) && type(event.value) == app.Manifest && event.kind == payday.CHANGE_KIND_CREATED && event.value.artifact_type == "application/vnd.dev.sigstore.bundle.v0.3+json"
}

func Example_dynamicConditions() {
	event := cz.Ident("event")
	conditions := []cz.Expr{
		cz.Eq(event.Select("kind"), cz.Ident("payday.CHANGE_KIND_CREATED")),
	}
	method := "/cr.index.Manifest/Put"
	if method != "" {
		conditions = append(conditions, cz.Eq(event.Select("method"), cz.String(method)))
	}
	filter, err := cz.Render(cz.And(conditions...))
	if err != nil {
		panic(err)
	}
	fmt.Println(filter)
	// Output: event.kind == payday.CHANGE_KIND_CREATED && event.method == "/cr.index.Manifest/Put"
}

func ExampleExpr_Call() {
	x := cz.Ident("x")
	expr := cz.Ident("items").Call("exists", x, cz.Gt(x.Select("price"), cz.Int(100)))
	source, err := cz.Render(expr)
	if err != nil {
		panic(err)
	}
	fmt.Println(source)
	// Output: items.exists(x, x.price > 100)
}

func ExampleMap() {
	expr := cz.Map(
		cz.Entry{Key: cz.String("name"), Value: cz.String("sample")},
		cz.Entry{Key: cz.String("enabled"), Value: cz.Bool(true)},
	)
	source, err := cz.Render(expr)
	if err != nil {
		panic(err)
	}
	fmt.Println(source)
	// Output: {"name": "sample", "enabled": true}
}
