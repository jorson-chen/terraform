package terraform

import (
	"reflect"
	"strings"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hcltest"
	"github.com/hashicorp/terraform/tfdiags"
	"github.com/zclconf/go-cty/cty"
)

func TestEvaluateResourceForEachExpression_valid(t *testing.T) {
	tests := map[string]struct {
		Expr       hcl.Expression
		ForEachMap map[string]cty.Value
	}{
		"empty set": {
			hcltest.MockExprLiteral(cty.SetValEmpty(cty.String)),
			map[string]cty.Value{},
		},
		"multi-value string set": {
			hcltest.MockExprLiteral(cty.SetVal([]cty.Value{cty.StringVal("a"), cty.StringVal("b")})),
			map[string]cty.Value{
				"a": cty.StringVal("a"),
				"b": cty.StringVal("b"),
			},
		},
		"empty map": {
			hcltest.MockExprLiteral(cty.MapValEmpty(cty.Bool)),
			map[string]cty.Value{},
		},
		"map": {
			hcltest.MockExprLiteral(cty.MapVal(map[string]cty.Value{
				"a": cty.BoolVal(true),
				"b": cty.BoolVal(false),
			})),
			map[string]cty.Value{
				"a": cty.BoolVal(true),
				"b": cty.BoolVal(false),
			},
		},
		"map containing unknown values": {
			hcltest.MockExprLiteral(cty.MapVal(map[string]cty.Value{
				"a": cty.UnknownVal(cty.Bool),
				"b": cty.UnknownVal(cty.Bool),
			})),
			map[string]cty.Value{
				"a": cty.UnknownVal(cty.Bool),
				"b": cty.UnknownVal(cty.Bool),
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctx := &MockEvalContext{}
			ctx.installSimpleEval()
			forEachMap, diags := evaluateResourceForEachExpression(test.Expr, ctx)

			if len(diags) != 0 {
				t.Errorf("unexpected diagnostics %s", spew.Sdump(diags))
			}

			if !reflect.DeepEqual(forEachMap, test.ForEachMap) {
				t.Errorf(
					"wrong map value\ngot:  %swant: %s",
					spew.Sdump(forEachMap), spew.Sdump(test.ForEachMap),
				)
			}

		})
	}
}

func TestEvaluateResourceForEachExpression_errors(t *testing.T) {
	tests := map[string]struct {
		Expr                     hcl.Expression
		Summary, DetailSubstring string
	}{
		"null set": {
			hcltest.MockExprLiteral(cty.NullVal(cty.Set(cty.String))),
			"Invalid for_each argument",
			`the given "for_each" argument value is null`,
		},
		"string": {
			hcltest.MockExprLiteral(cty.StringVal("i am definitely a set")),
			"Invalid for_each argument",
			"must be a map, or set of strings, and you have provided a value of type string",
		},
		"list": {
			hcltest.MockExprLiteral(cty.ListVal([]cty.Value{cty.StringVal("a"), cty.StringVal("a")})),
			"Invalid for_each argument",
			"must be a map, or set of strings, and you have provided a value of type list",
		},
		"tuple": {
			hcltest.MockExprLiteral(cty.TupleVal([]cty.Value{cty.StringVal("a"), cty.StringVal("b")})),
			"Invalid for_each argument",
			"must be a map, or set of strings, and you have provided a value of type tuple",
		},
		"unknown string set": {
			hcltest.MockExprLiteral(cty.UnknownVal(cty.Set(cty.String))),
			"Invalid for_each argument",
			"depends on resource attributes that cannot be determined until apply",
		},
		"unknown map": {
			hcltest.MockExprLiteral(cty.UnknownVal(cty.Map(cty.Bool))),
			"Invalid for_each argument",
			"depends on resource attributes that cannot be determined until apply",
		},
		"set containing booleans": {
			hcltest.MockExprLiteral(cty.SetVal([]cty.Value{cty.BoolVal(true)})),
			"Invalid for_each set argument",
			"supports maps and sets of strings, but you have provided a set containing type bool",
		},
		"set containing null": {
			hcltest.MockExprLiteral(cty.SetVal([]cty.Value{cty.NullVal(cty.String)})),
			"Invalid for_each set argument",
			"must not contain null values",
		},
		"set containing unknown value": {
			hcltest.MockExprLiteral(cty.SetVal([]cty.Value{cty.UnknownVal(cty.String)})),
			"Invalid for_each argument",
			"depends on resource attributes that cannot be determined until apply",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctx := &MockEvalContext{}
			ctx.installSimpleEval()
			_, diags := evaluateResourceForEachExpression(test.Expr, ctx)

			if len(diags) != 1 {
				t.Fatalf("got %d diagnostics; want 1", diags)
			}
			if got, want := diags[0].Severity(), tfdiags.Error; got != want {
				t.Errorf("wrong diagnostic severity %#v; want %#v", got, want)
			}
			if got, want := diags[0].Description().Summary, test.Summary; got != want {
				t.Errorf("wrong diagnostic summary %#v; want %#v", got, want)
			}
			if got, want := diags[0].Description().Detail, test.DetailSubstring; !strings.Contains(got, want) {
				t.Errorf("wrong diagnostic detail %#v; want %#v", got, want)
			}
		})
	}
}

func TestEvaluateResourceForEachExpressionKnown(t *testing.T) {
	tests := map[string]hcl.Expression{
		"unknown string set": hcltest.MockExprLiteral(cty.UnknownVal(cty.Set(cty.String))),
		"unknown map":        hcltest.MockExprLiteral(cty.UnknownVal(cty.Map(cty.Bool))),
	}

	for name, expr := range tests {
		t.Run(name, func(t *testing.T) {
			ctx := &MockEvalContext{}
			ctx.installSimpleEval()
			forEachMap, known, diags := evaluateResourceForEachExpressionKnown(expr, ctx)

			if len(diags) != 0 {
				t.Errorf("unexpected diagnostics %s", spew.Sdump(diags))
			}

			if known {
				t.Errorf("got %v known, want false", known)
			}

			if len(forEachMap) != 0 {
				t.Errorf(
					"expected empty map\ngot:  %s",
					spew.Sdump(forEachMap),
				)
			}

		})
	}
}
