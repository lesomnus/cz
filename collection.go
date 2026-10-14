package cz

import (
	"fmt"
	"slices"

	"cel.dev/cel-go/common/ast"
)

// List creates a CEL list literal. The input slice is copied.
func List(elements ...Expr) Expr {
	elements = slices.Clone(elements)
	return Expr{build: func(b *builder) (ast.Expr, error) {
		values, err := b.exprs(elements)
		if err != nil {
			return nil, err
		}
		return b.factory.NewList(b.id(), values, nil), nil
	}}
}

// Entry is a key/value pair in a Map literal.
type Entry struct {
	Key   Expr
	Value Expr
}

// Map creates a CEL map literal. Entries are copied and retain their order for
// deterministic output. Key types and duplicates are checked by CEL when the
// rendered source is compiled or evaluated.
func Map(entries ...Entry) Expr {
	entries = slices.Clone(entries)
	return Expr{build: func(b *builder) (ast.Expr, error) {
		values := make([]ast.EntryExpr, len(entries))
		for i, entry := range entries {
			key, err := b.expr(entry.Key)
			if err != nil {
				return nil, fmt.Errorf("map entry %d key: %w", i, err)
			}
			value, err := b.expr(entry.Value)
			if err != nil {
				return nil, fmt.Errorf("map entry %d value: %w", i, err)
			}
			values[i] = b.factory.NewMapEntry(b.id(), key, value, false)
		}
		return b.factory.NewMap(b.id(), values), nil
	}}
}
