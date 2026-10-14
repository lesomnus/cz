package cz

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"cel.dev/cel-go/common/ast"
	"cel.dev/cel-go/common/operators"
	"cel.dev/cel-go/parser"
)

// Expr is an immutable CEL expression. Its zero value is invalid.
type Expr struct {
	build func(*builder) (ast.Expr, error)
}

type builder struct {
	factory ast.ExprFactory
	lastID  int64
}

func (b *builder) id() int64 {
	b.lastID++
	return b.lastID
}

func (b *builder) expr(e Expr) (ast.Expr, error) {
	if e.build == nil {
		return nil, fmt.Errorf("cz: zero Expr; use a literal or expression constructor")
	}
	return e.build(b)
}

func (b *builder) exprs(exprs []Expr) ([]ast.Expr, error) {
	out := make([]ast.Expr, len(exprs))
	for i, expr := range exprs {
		built, err := b.expr(expr)
		if err != nil {
			return nil, fmt.Errorf("argument %d: %w", i, err)
		}
		out[i] = built
	}
	return out, nil
}

func (b *builder) postfixOperand(expr ast.Expr) ast.Expr {
	if expr.Kind() != ast.CallKind || operators.Arity(expr.AsCall().FunctionName()) != 1 {
		return expr
	}
	// cel-go's unparser groups binary/ternary receivers, but not unary ones:
	// (!flag).asInt() would become !flag.asInt(), changing its meaning.
	// An unnamed call emits only parentheses around its argument. This is a
	// rendering-only grouping node; our AST is never passed to a CEL checker.
	return b.factory.NewCall(b.id(), "", expr)
}

func invalid(format string, args ...any) Expr {
	err := fmt.Errorf("cz: "+format, args...)
	return Expr{build: func(*builder) (ast.Expr, error) { return nil, err }}
}

// Render returns CEL source, preserving operator precedence and escaping
// literals. It reports invalid names, literal values, zero expressions, and
// invalid Has operands. It does not check types or function/macro signatures;
// compile the returned source in your CEL environment for those checks.
func Render(expr Expr) (string, error) {
	b := &builder{factory: ast.NewExprFactory()}
	root, err := b.expr(expr)
	if err != nil {
		return "", err
	}
	text, err := parser.Unparse(root, nil, parser.WrapOnOperators())
	if err != nil {
		return "", fmt.Errorf("cz: render: %w", err)
	}
	return text, nil
}

// Ident names a CEL symbol, such as "event", "app.Manifest", or
// "payday.CHANGE_KIND_CREATED". A leading dot denotes an absolute name.
// It does not insert raw CEL source. Use String for string values and Select
// for field access on an expression.
func Ident(name string) Expr {
	if !validName(name, true) {
		return invalid("invalid identifier %q", name)
	}
	return Expr{build: func(b *builder) (ast.Expr, error) {
		return b.factory.NewIdent(b.id(), name), nil
	}}
}

// Select accesses a single field on e. Chain calls for nested fields.
// Fields must be unquoted CEL identifiers. Use Index(String(key)) for map keys
// that contain punctuation or reserved literal names.
func (e Expr) Select(field string) Expr {
	if !validField(field) {
		return invalid("invalid field %q; use Index(String(key)) for arbitrary map keys", field)
	}
	return Expr{build: func(b *builder) (ast.Expr, error) {
		target, err := b.expr(e)
		if err != nil {
			return nil, err
		}
		return b.factory.NewSelect(b.id(), b.postfixOperand(target), field), nil
	}}
}

// Index accesses a list element or map entry.
func (e Expr) Index(key Expr) Expr { return operation(operators.Index, e, key) }

// Call invokes a global function or macro. Qualified names are allowed, such as
// "math.greatest". Register any required extensions in the CEL environment
// that compiles the rendered source.
func Call(name string, args ...Expr) Expr {
	if !validName(name, true) || operators.Arity(name) >= 0 {
		return invalid("invalid function name %q", name)
	}
	return operation(name, args...)
}

// Call invokes a receiver function or macro, such as value.startsWith(prefix)
// or items.exists(x, x > 0). The name must be a single identifier.
func (e Expr) Call(name string, args ...Expr) Expr {
	if !validName(name, false) || operators.Arity(name) >= 0 {
		return invalid("invalid receiver function name %q", name)
	}
	args = slices.Clone(args)
	return Expr{build: func(b *builder) (ast.Expr, error) {
		target, err := b.expr(e)
		if err != nil {
			return nil, err
		}
		values, err := b.exprs(args)
		if err != nil {
			return nil, err
		}
		return b.factory.NewMemberCall(b.id(), name, b.postfixOperand(target), values...), nil
	}}
}

func operation(name string, args ...Expr) Expr {
	args = slices.Clone(args)
	return Expr{build: func(b *builder) (ast.Expr, error) {
		values, err := b.exprs(args)
		if err != nil {
			return nil, err
		}
		if name == operators.Index {
			values[0] = b.postfixOperand(values[0])
		}
		return b.factory.NewCall(b.id(), name, values...), nil
	}}
}

var identifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func validField(name string) bool {
	if !identifier.MatchString(name) {
		return false
	}
	switch name {
	case "true", "false", "null", "in":
		return false
	}
	return true
}

func validName(name string, qualified bool) bool {
	if qualified {
		name = strings.TrimPrefix(name, ".")
	} else if strings.Contains(name, ".") {
		return false
	}
	for _, part := range strings.Split(name, ".") {
		if !validField(part) {
			return false
		}
		switch part {
		case "as", "break", "const", "continue", "else", "for", "function", "if",
			"import", "let", "loop", "package", "namespace", "return", "var", "void", "while":
			return false
		}
	}
	return true
}
