package cz_test

import (
	"fmt"
	"math"
	"strings"
	"testing"
	"unicode/utf8"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
	"github.com/lesomnus/cz"
)

func evaluate(t testing.TB, expr cz.Expr, variables map[string]any, options ...cel.EnvOption) ref.Val {
	t.Helper()
	source, err := cz.Render(expr)
	if err != nil {
		t.Fatal(err)
	}
	env, err := cel.NewEnv(options...)
	if err != nil {
		t.Fatal(err)
	}
	compiled, issues := env.Compile(source)
	if issues.Err() != nil {
		t.Fatalf("compile %q: %v", source, issues.Err())
	}
	program, err := env.Program(compiled)
	if err != nil {
		t.Fatal(err)
	}
	if variables == nil {
		variables = map[string]any{}
	}
	out, _, err := program.Eval(variables)
	if err != nil {
		t.Fatalf("evaluate %q: %v", source, err)
	}
	return out
}

func TestOperators(t *testing.T) {
	a, b, c := cz.Int(20), cz.Int(6), cz.Int(2)
	tests := []struct {
		name string
		expr cz.Expr
		want ref.Val
	}{
		{"equality", cz.Eq(a, a), types.True},
		{"inequality", cz.Ne(a, b), types.True},
		{"less", cz.Lt(c, b), types.True},
		{"less equal", cz.Le(b, b), types.True},
		{"greater", cz.Gt(a, b), types.True},
		{"greater equal", cz.Ge(a, a), types.True},
		{"and", cz.And(cz.Bool(true), cz.Bool(false), cz.Bool(true)), types.False},
		{"or", cz.Or(cz.Bool(false), cz.Bool(true), cz.Bool(false)), types.True},
		{"empty and", cz.And(), types.True},
		{"empty or", cz.Or(), types.False},
		{"one and", cz.And(cz.Bool(false)), types.False},
		{"one or", cz.Or(cz.Bool(true)), types.True},
		{"not", cz.Not(cz.Bool(false)), types.True},
		{"double not", cz.Not(cz.Not(cz.Bool(true))), types.True},
		{"addition", cz.Add(a, b), types.Int(26)},
		{"subtraction", cz.Sub(a, cz.Sub(b, c)), types.Int(16)},
		{"multiplication", cz.Mul(cz.Add(a, b), c), types.Int(52)},
		{"division", cz.Div(a, cz.Div(b, c)), types.Int(6)},
		{"modulo", cz.Mod(a, b), types.Int(2)},
		{"negation", cz.Neg(cz.Add(a, b)), types.Int(-26)},
		{"double negation", cz.Neg(cz.Neg(c)), types.Int(2)},
		{"negate negative literal", cz.Neg(cz.Int(-2)), types.Int(2)},
		{"conditional", cz.If(cz.Lt(a, b), a, b), types.Int(6)},
		{"nested conditional", cz.If(cz.Bool(true), cz.If(cz.Bool(false), a, b), c), types.Int(6)},
		{"conditional in addition", cz.Add(cz.If(cz.Bool(false), a, b), c), types.Int(8)},
		{"logical precedence", cz.And(cz.Or(cz.Bool(true), cz.Bool(false)), cz.Bool(false)), types.False},
		{"membership", cz.In(c, cz.List(a, b, c)), types.True},
		{"index", cz.List(a, b, c).Index(cz.Int(1)), types.Int(6)},
		{"index after conditional", cz.If(cz.Bool(true), cz.List(a), cz.List(b)).Index(cz.Int(0)), types.Int(20)},
		{"receiver after addition", cz.Add(cz.String("ab"), cz.String("cd")).Call("endsWith", cz.String("cd")), types.True},
		{"global call", cz.Call("size", cz.List(a, b, c)), types.Int(3)},
		{"type", cz.Eq(cz.Type(a), cz.Ident("int")), types.True},
		{"empty list", cz.Call("size", cz.List()), types.Int(0)},
		{"empty map", cz.Call("size", cz.Map()), types.Int(0)},
		{"absolute name", cz.Eq(cz.Ident(".int"), cz.Type(a)), types.True},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evaluate(t, tt.expr, nil)
			if got.Equal(tt.want) != types.True {
				t.Fatalf("got %v (%v), want %v (%v)", got, got.Type(), tt.want, tt.want.Type())
			}
		})
	}
}

func TestMapAndPresence(t *testing.T) {
	mapping := cz.Map(
		cz.Entry{Key: cz.String("artifact_type"), Value: cz.String("image")},
		cz.Entry{Key: cz.String("a.b\"\\"), Value: cz.Int(42)},
		cz.Entry{Key: cz.String("for"), Value: cz.Bool(true)},
	)
	value := cz.If(cz.Bool(true), mapping, cz.Map())
	expr := cz.And(
		cz.Has(value.Select("artifact_type")),
		cz.Not(cz.Has(value.Select("missing"))),
		cz.Eq(value.Select("artifact_type"), cz.String("image")),
		cz.Eq(value.Index(cz.String("a.b\"\\")), cz.Int(42)),
		value.Select("for"),
	)
	if got := evaluate(t, expr, nil); got != types.True {
		t.Fatalf("got %v", got)
	}
}

func TestMacrosAndExtensions(t *testing.T) {
	x := cz.Ident("x")
	list := cz.List(cz.Int(1), cz.Int(2), cz.Int(3))
	expr := cz.And(
		list.Call("all", x, cz.Gt(x, cz.Int(0))),
		list.Call("exists", x, cz.Eq(x, cz.Int(2))),
		list.Call("exists_one", x, cz.Eq(x, cz.Int(3))),
		cz.Eq(list.Call("filter", x, cz.Gt(x, cz.Int(1))), cz.List(cz.Int(2), cz.Int(3))),
		cz.Eq(list.Call("map", x, cz.Mul(x, cz.Int(2))), cz.List(cz.Int(2), cz.Int(4), cz.Int(6))),
		cz.Call("example.isEven", cz.Int(42)),
	)
	fn := cel.Function("example.isEven", cel.Overload("example_is_even_int", []*cel.Type{cel.IntType}, cel.BoolType,
		cel.UnaryBinding(func(value ref.Val) ref.Val { return types.Bool(value.(types.Int)%2 == 0) })))
	if got := evaluate(t, expr, nil, fn); got != types.True {
		t.Fatalf("got %v", got)
	}
}

func TestUnaryAndNumericReceivers(t *testing.T) {
	square := cel.Function("square", cel.MemberOverload("int_square", []*cel.Type{cel.IntType}, cel.IntType,
		cel.UnaryBinding(func(value ref.Val) ref.Val {
			n := value.(types.Int)
			return n * n
		})))
	asInt := cel.Function("asInt", cel.MemberOverload("bool_as_int", []*cel.Type{cel.BoolType}, cel.IntType,
		cel.UnaryBinding(func(value ref.Val) ref.Val {
			if value == types.True {
				return types.Int(1)
			}
			return types.Int(0)
		})))
	for _, expr := range []cz.Expr{
		cz.Int(2).Call("square"),
		cz.Int(-2).Call("square"),
		cz.Neg(cz.Int(2)).Call("square"),
		cz.Neg(cz.Neg(cz.Int(2))).Call("square"),
		cz.Neg(cz.Ident("n")).Call("square"),
	} {
		if got := evaluate(t, expr, map[string]any{"n": int64(2)}, square, cel.Variable("n", cel.IntType)); got != types.Int(4) {
			t.Fatalf("got %v, want 4", got)
		}
	}
	if got := evaluate(t, cz.Not(cz.Bool(false)).Call("asInt"), nil, asInt); got != types.Int(1) {
		t.Fatalf("got %v, want 1", got)
	}
}

func TestLiteralRoundTrip(t *testing.T) {
	tests := []struct {
		value any
		want  ref.Val
	}{
		{nil, types.NullValue}, {true, types.True}, {false, types.False},
		{"", types.String("")}, {"한글\n\t\"\\\x00", types.String("한글\n\t\"\\\x00")},
		{`\" || true || \"`, types.String(`\" || true || \"`)},
		{[]byte{0, 255, '"', '\\', '\n'}, types.Bytes{0, 255, '"', '\\', '\n'}},
		{[]byte(nil), types.Bytes{}},
		{int(-1), types.Int(-1)}, {int8(-128), types.Int(-128)},
		{int16(-32768), types.Int(-32768)}, {int32(-2147483648), types.Int(-2147483648)},
		{int64(math.MinInt64), types.Int(math.MinInt64)}, {int64(math.MaxInt64), types.Int(math.MaxInt64)},
		{uint(1), types.Uint(1)}, {uint8(255), types.Uint(255)},
		{uint16(65535), types.Uint(65535)}, {uint32(4294967295), types.Uint(4294967295)},
		{uint64(math.MaxUint64), types.Uint(math.MaxUint64)},
		{float32(1.25), types.Double(1.25)}, {float64(1), types.Double(1)},
		{math.SmallestNonzeroFloat64, types.Double(math.SmallestNonzeroFloat64)},
		{math.MaxFloat64, types.Double(math.MaxFloat64)},
	}
	for i, tt := range tests {
		t.Run(fmt.Sprintf("%d/%T", i, tt.value), func(t *testing.T) {
			got := evaluate(t, cz.Literal(tt.value), nil)
			if got.Equal(tt.want) != types.True || got.Type() != tt.want.Type() {
				t.Fatalf("got %v (%v), want %v (%v)", got, got.Type(), tt.want, tt.want.Type())
			}
		})
	}
	t.Run("negative zero", func(t *testing.T) {
		got := evaluate(t, cz.Double(math.Copysign(0, -1)), nil).(types.Double)
		if !math.Signbit(float64(got)) {
			t.Fatal("lost the sign of negative zero")
		}
	})
}

func TestInvalidExpressions(t *testing.T) {
	tests := []cz.Expr{
		{}, cz.And(cz.Bool(true), cz.Expr{}), cz.Or(cz.Expr{}),
		cz.Ident(""), cz.Ident("."), cz.Ident("..event"), cz.Ident("a..b"),
		cz.Ident("event || true"), cz.Ident("true"), cz.Ident("if"), cz.Ident("event.1value"),
		cz.Ident("a").Select("value.foo"), cz.Ident("a").Select("value`) || true"),
		cz.Ident("a").Select("in"), cz.Ident("a").Select("null"), cz.Expr{}.Select("value"),
		cz.Call("x) || true || x("), cz.Call("_in_"), cz.Call("has", cz.Expr{}),
		cz.Expr{}.Call("size"), cz.Int(1).Call("a.b"), cz.Int(1).Call("_in_"),
		cz.Int(1).Call("x", cz.Expr{}), cz.Call("x", cz.Expr{}),
		cz.Has(cz.Int(1)), cz.Has(cz.Ident("event.value")), cz.Has(cz.Expr{}),
		cz.String(string([]byte{0xff})), cz.Double(math.NaN()), cz.Double(math.Inf(1)), cz.Double(math.Inf(-1)),
		cz.Literal(struct{}{}), cz.Literal([]int{1}), cz.Literal(uintptr(1)),
		cz.List(cz.Expr{}), cz.Map(cz.Entry{Value: cz.Int(1)}), cz.Map(cz.Entry{Key: cz.String("x")}),
	}
	for i, expr := range tests {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			text, err := cz.Render(expr)
			if err == nil || text != "" {
				t.Fatalf("got (%q, %v), want empty source and an error", text, err)
			}
		})
	}
}

func TestTypeCheckingBelongsToEnvironment(t *testing.T) {
	source, err := cz.Render(cz.Add(cz.String("hello"), cz.Int(1)))
	if err != nil {
		t.Fatal(err)
	}
	env, err := cel.NewEnv()
	if err != nil {
		t.Fatal(err)
	}
	if _, issues := env.Compile(source); issues.Err() == nil {
		t.Fatal("expected CEL to reject string + int")
	}
}

func TestInputsAreCopied(t *testing.T) {
	data := []byte{1, 2, 3}
	bytesExpr := cz.Bytes(data)
	data[0] = 255
	args := []cz.Expr{cz.Int(1), cz.Int(2)}
	list := cz.List(args...)
	call := cz.Call("f", args...)
	member := cz.Ident("x").Call("f", args...)
	args[0] = cz.Expr{}
	conditions := []cz.Expr{cz.Bool(true), cz.Bool(false)}
	and := cz.And(conditions...)
	conditions[0] = cz.Expr{}
	entries := []cz.Entry{{Key: cz.String("a"), Value: cz.Int(1)}}
	mapping := cz.Map(entries...)
	entries[0] = cz.Entry{}
	expr := cz.And(
		cz.Eq(bytesExpr, cz.Bytes([]byte{1, 2, 3})),
		cz.Eq(list, cz.List(cz.Int(1), cz.Int(2))),
		cz.Not(and),
		cz.Eq(mapping.Select("a"), cz.Int(1)),
	)
	if got := evaluate(t, expr, nil); got != types.True {
		t.Fatalf("got %v", got)
	}
	for _, tc := range []struct {
		expr cz.Expr
		want string
	}{{call, "f(1, 2)"}, {member, "x.f(1, 2)"}} {
		text, err := cz.Render(tc.expr)
		if err != nil || text != tc.want {
			t.Fatalf("got (%q, %v), want %q", text, err, tc.want)
		}
	}
}

func TestConcurrentReuse(t *testing.T) {
	x := cz.Ident("x")
	shared := cz.Gt(x, cz.Int(10))
	expr := cz.And(shared, cz.Or(cz.Bool(false), shared))
	want, err := cz.Render(expr)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 16; i++ {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			t.Parallel()
			for j := 0; j < 20; j++ {
				got, err := cz.Render(expr)
				if err != nil || got != want {
					t.Fatalf("got (%q, %v), want %q", got, err, want)
				}
			}
			got := evaluate(t, expr, map[string]any{"x": int64(11)}, cel.Variable("x", cel.IntType))
			if got != types.True {
				t.Fatalf("got %v", got)
			}
		})
	}
}

func TestLargeConditionList(t *testing.T) {
	conditions := make([]cz.Expr, 1000)
	for i := range conditions {
		conditions[i] = cz.Eq(cz.Int(int64(i)), cz.Int(int64(i)))
	}
	if got := evaluate(t, cz.And(conditions...), nil); got != types.True {
		t.Fatalf("got %v", got)
	}
}

func FuzzStringLiteral(f *testing.F) {
	for _, seed := range []string{"", "hello", "한글", "\x00\n\r\t\"\\", `\" || true || \"`, string([]byte{0xff}), "\u2028\U0001f680"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, value string) {
		if len(value) > 8192 {
			t.Skip()
		}
		expr := cz.String(value)
		if !utf8.ValidString(value) {
			if _, err := cz.Render(expr); err == nil {
				t.Fatal("accepted invalid UTF-8")
			}
			return
		}
		if got := evaluate(t, expr, nil); got != types.String(value) {
			t.Fatalf("string did not round trip: %q", value)
		}
	})
}

func FuzzBytesLiteral(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0, 255, '\\', '"', '\n'})
	f.Add([]byte(strings.Repeat("x", 64)))
	f.Fuzz(func(t *testing.T, value []byte) {
		if len(value) > 8192 {
			t.Skip()
		}
		if got := evaluate(t, cz.Bytes(value), nil); got.Equal(types.Bytes(value)) != types.True {
			t.Fatalf("bytes did not round trip: %x", value)
		}
	})
}
