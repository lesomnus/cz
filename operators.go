package cz

import (
	"fmt"

	"cel.dev/cel-go/common/ast"
	"cel.dev/cel-go/common/operators"
)

// Eq compares a and b for equality.
func Eq(a, b Expr) Expr { return operation(operators.Equals, a, b) }

// Ne compares a and b for inequality.
func Ne(a, b Expr) Expr { return operation(operators.NotEquals, a, b) }

// Lt builds a < b.
func Lt(a, b Expr) Expr { return operation(operators.Less, a, b) }

// Le builds a <= b.
func Le(a, b Expr) Expr { return operation(operators.LessEquals, a, b) }

// Gt builds a > b.
func Gt(a, b Expr) Expr { return operation(operators.Greater, a, b) }

// Ge builds a >= b.
func Ge(a, b Expr) Expr { return operation(operators.GreaterEquals, a, b) }

// And joins expressions with &&. With no arguments it returns true; with one
// argument it returns that expression. It does not discard invalid arguments.
func And(exprs ...Expr) Expr { return logical(operators.LogicalAnd, true, exprs) }

// Or joins expressions with ||. With no arguments it returns false; with one
// argument it returns that expression. It does not discard invalid arguments.
func Or(exprs ...Expr) Expr { return logical(operators.LogicalOr, false, exprs) }

func logical(name string, identity bool, exprs []Expr) Expr {
	switch len(exprs) {
	case 0:
		return Bool(identity)
	case 1:
		return exprs[0]
	}
	// A balanced tree keeps construction and rendering stack depth logarithmic
	// for large condition lists while preserving operand order.
	mid := len(exprs) / 2
	return operation(name,
		logical(name, identity, exprs[:mid]),
		logical(name, identity, exprs[mid:]),
	)
}

// Not negates a boolean expression.
func Not(expr Expr) Expr { return operation(operators.LogicalNot, expr) }

// Add builds a + b (numeric addition or string/list concatenation).
func Add(a, b Expr) Expr { return operation(operators.Add, a, b) }

// Sub builds a - b.
func Sub(a, b Expr) Expr { return operation(operators.Subtract, a, b) }

// Mul builds a * b.
func Mul(a, b Expr) Expr { return operation(operators.Multiply, a, b) }

// Div builds a / b.
func Div(a, b Expr) Expr { return operation(operators.Divide, a, b) }

// Mod builds a % b.
func Mod(a, b Expr) Expr { return operation(operators.Modulo, a, b) }

// Neg builds -expr.
func Neg(expr Expr) Expr { return operation(operators.Negate, expr) }

// In tests list membership or map key presence.
func In(value, collection Expr) Expr { return operation(operators.In, value, collection) }

// If builds condition ? whenTrue : whenFalse.
func If(condition, whenTrue, whenFalse Expr) Expr {
	return operation(operators.Conditional, condition, whenTrue, whenFalse)
}

// Has tests field presence, for example Has(Ident("event").Select("value")).
// Its argument must be a Select expression. For a map key supplied as data,
// use In(String(key), mapExpr).
func Has(field Expr) Expr {
	return Expr{build: func(b *builder) (ast.Expr, error) {
		selected, err := b.expr(field)
		if err != nil {
			return nil, err
		}
		if selected.Kind() != ast.SelectKind {
			return nil, fmt.Errorf("cz: Has requires a Select expression")
		}
		// Keep the macro call until the receiving CEL parser expands it. This
		// also lets Unparse parenthesize a complex selection operand correctly.
		return b.factory.NewCall(b.id(), operators.Has, selected), nil
	}}
}

// Type returns the CEL type of expr, as in type(event.value).
func Type(expr Expr) Expr { return Call("type", expr) }
