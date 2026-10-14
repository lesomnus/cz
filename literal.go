package cz

import (
	"math"
	"slices"
	"unicode/utf8"

	"cel.dev/cel-go/common/ast"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
)

func literal(value ref.Val) Expr {
	return Expr{build: func(b *builder) (ast.Expr, error) {
		return b.factory.NewLiteral(b.id(), value), nil
	}}
}

// String creates a CEL string literal. Invalid UTF-8 is rejected by Render.
func String(value string) Expr {
	if !utf8.ValidString(value) {
		return invalid("string is not valid UTF-8; use Bytes for binary data")
	}
	return literal(types.String(value))
}

// Bytes creates a CEL bytes literal, copying value so later changes do not
// affect the expression. A nil slice represents empty bytes.
func Bytes(value []byte) Expr { return literal(types.Bytes(slices.Clone(value))) }

// Bool creates a CEL bool literal.
func Bool(value bool) Expr { return literal(types.Bool(value)) }

// Int creates a CEL signed 64-bit integer literal.
func Int(value int64) Expr { return literal(types.Int(value)) }

// Uint creates a CEL unsigned 64-bit integer literal.
func Uint(value uint64) Expr { return literal(types.Uint(value)) }

// Double creates a CEL double literal. Render rejects NaN and infinities,
// which have no CEL literal syntax. Use Call("double", String("NaN")) or
// Call("double", String("Infinity")) when such values are needed.
func Double(value float64) Expr {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return invalid("non-finite double has no CEL literal syntax")
	}
	return literal(types.Double(value))
}

// Null creates a CEL null literal.
func Null() Expr { return literal(types.NullValue) }

// Literal converts a Go scalar into a CEL literal. It accepts nil, bool,
// string, []byte, all built-in signed and unsigned integer types except
// uintptr, and float32/float64. Named types require an explicit conversion.
// Use List and Map for collections. Unsupported types are reported by Render.
func Literal(value any) Expr {
	switch v := value.(type) {
	case nil:
		return Null()
	case bool:
		return Bool(v)
	case string:
		return String(v)
	case []byte:
		return Bytes(v)
	case int:
		return Int(int64(v))
	case int8:
		return Int(int64(v))
	case int16:
		return Int(int64(v))
	case int32:
		return Int(int64(v))
	case int64:
		return Int(v)
	case uint:
		return Uint(uint64(v))
	case uint8:
		return Uint(uint64(v))
	case uint16:
		return Uint(uint64(v))
	case uint32:
		return Uint(uint64(v))
	case uint64:
		return Uint(v)
	case float32:
		return Double(float64(v))
	case float64:
		return Double(v)
	default:
		return invalid("unsupported literal type %T", value)
	}
}
