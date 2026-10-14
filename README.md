# cz

Build [CEL](https://cel.dev/) expressions in Go.

`cz` assembles immutable expressions and renders CEL source using
[cel-go's AST and unparser](https://pkg.go.dev/cel.dev/cel-go/parser#Unparse).
It handles literal escaping, operator precedence, and internal node IDs.
It has no dependency on payday or any application schema.

## Install

Requires Go 1.23 or later.

```sh
go get github.com/lesomnus/cz
```

## Build an expression

```go
import "github.com/lesomnus/cz"

user := cz.Ident("user")
expr := cz.And(
    cz.Ge(user.Select("age"), cz.Int(18)),
    cz.In(user.Select("country"), cz.List(cz.String("KR"), cz.String("US"))),
)

source, err := cz.Render(expr)
// user.age >= 18 && user.country in ["KR", "US"]
```

Handle `err` before using the source. Constructors defer errors until `Render`
so expressions can be composed without an error check at every step.

`Ident("app.Manifest")` is a CEL symbol; `String("app.Manifest")` is a string
value. Pass application data through literal constructors. Names are validated
as identifiers, and string/bytes values are escaped by cel-go. `cz` does not
provide a raw-source interpolation API.

## Compose conditions

```go
conditions := []cz.Expr{cz.Ge(user.Select("age"), cz.Int(18))}
if country != "" {
    conditions = append(conditions,
        cz.Eq(user.Select("country"), cz.String(country)),
    )
}
source, err := cz.Render(cz.And(conditions...))
```

`And()` is `true`; `Or()` is `false`. With one argument, both return that
expression. A zero `cz.Expr{}` is an error, including inside a condition list.

Expressions can be reused and rendered concurrently. Constructors copy input
slices, including bytes and list/map entries, so later changes do not alter an
existing expression.

## Functions and macros

```go
prefix := cz.Ident("name").Call("startsWith", cz.String("release-"))
length := cz.Call("size", cz.Ident("items"))

x := cz.Ident("x")
anyExpensive := cz.Ident("items").Call("exists", x,
    cz.Gt(x.Select("price"), cz.Int(100)),
)
// items.exists(x, x.price > 100)
```

`Call` and `Expr.Call` also support custom functions and macros available in
the receiving CEL environment. For example, use `cz.Call("timestamp",
cz.String("2026-01-01T00:00:00Z"))` or `cz.Call("duration", cz.String("5m"))`.
Macros expand when the receiving CEL parser compiles the rendered source.

## API

| Purpose | Constructors and methods |
| --- | --- |
| Symbols and fields | `Ident`, `Expr.Select`, `Expr.Index` |
| Literals | `String`, `Bytes`, `Bool`, `Int`, `Uint`, `Double`, `Null`, `Literal` |
| Comparisons | `Eq`, `Ne`, `Lt`, `Le`, `Gt`, `Ge` |
| Logic | `And`, `Or`, `Not` |
| Arithmetic | `Add`, `Sub`, `Mul`, `Div`, `Mod`, `Neg` |
| Collections | `List`, `Map`, `Entry`, `In` |
| Conditional | `If(condition, whenTrue, whenFalse)` |
| Presence and types | `Has`, `Type` |
| Functions and macros | `Call`, `Expr.Call` |
| Output | `Render` |

`Select` takes one unquoted field name; chain it for nested fields. For arbitrary
map keys, use `value.Index(cz.String(key))`. `Has` requires a `Select` expression;
use `In(cz.String(key), value)` to test a map key supplied as data.

`Literal` accepts built-in Go scalars and `[]byte`. Convert named types
explicitly. Collections use `List` and ordered `Map` entries. Invalid UTF-8
strings and non-finite double literals are rejected. Protobuf object literals
and CEL optional-access operators are not part of the initial API.

## Compile and evaluate

`Render` checks construction errors; it does not resolve declarations or check
CEL types and function/macro signatures. Compile the source in the environment
that will evaluate it:

```go
env, err := cel.NewEnv(cel.Variable("user", cel.MapType(cel.StringType, cel.DynType)))
if err != nil {
    return err
}
checked, issues := env.Compile(source)
if issues.Err() != nil {
    return issues.Err()
}
program, err := env.Program(checked)
if err != nil {
    return err
}
result, _, err := program.Eval(map[string]any{
    "user": map[string]any{"age": int64(25), "country": "KR"},
})
```

Here `cel` is `cel.dev/cel-go/cel`. For a remote filter API, send `source` to
that API and let its CEL environment compile and evaluate the expression.

## Example: payday Watch

Application-specific identifiers stay in the caller:

```go
event := cz.Ident("event")
value := event.Select("value")

expr := cz.And(
    cz.Has(value),
    cz.Eq(cz.Type(value), cz.Ident("app.Manifest")),
    cz.Eq(event.Select("kind"), cz.Ident("payday.CHANGE_KIND_CREATED")),
    cz.Eq(value.Select("artifact_type"), cz.String(artifactType)),
)

if method != "" {
    expr = cz.And(expr,
        cz.Eq(event.Select("method"), cz.String(method)),
    )
}

filter, err := cz.Render(expr)
if err != nil {
    return err
}
req := &pdpb.WatchRequest{Filter: filter}
```

The presence and type guards handle events whose `Any` payload is absent or
contains a different resource. The same builder works with any CEL environment;
`cz` does not register these application types or enums.

## Development

```sh
go test ./...
go vet ./...
go test -race ./...
go test -run '^$' -fuzz '^FuzzStringLiteral$' -fuzztime 10s
go test -run '^$' -fuzz '^FuzzBytesLiteral$' -fuzztime 10s
```

Tests compile and evaluate generated CEL, including protobuf `Any` filters,
operator grouping, macros, literal round trips, and concurrent expression reuse.

## License

Apache-2.0. See [LICENSE](LICENSE).
