// Package cz builds CEL expressions in Go and renders them as CEL source.
//
// Expressions are immutable and may be reused, composed, and rendered
// concurrently. Constructors defer errors until Render so that expressions can
// be assembled without checking an error after every operation. The zero Expr
// is invalid; use Bool(true) for an unconditional predicate.
//
// Render does not type-check expressions. Compile its output in the receiving
// CEL environment to validate variable declarations, types, functions, and
// macros. Pass application data through literal constructors such as String;
// Ident names a CEL symbol, not a string value.
package cz
