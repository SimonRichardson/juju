// Copyright 2026 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package assumes

import (
	"testing"

	"github.com/juju/tc"
)

type RequiredSuite struct{}

func TestRequiredSuite(t *testing.T) {
	tc.Run(t, &RequiredSuite{})
}

func (s *RequiredSuite) TestRequiresFeature(c *tc.C) {
	testCases := []struct {
		about   string
		tree    *ExpressionTree
		feature string
		expect  bool
	}{
		{
			about:   "nil tree",
			feature: "holistic-uniter",
		},
		{
			about: "direct feature",
			tree: &ExpressionTree{Expression: FeatureExpression{
				Name: "holistic-uniter",
			}},
			feature: "holistic-uniter",
			expect:  true,
		},
		{
			about: "all of includes feature",
			tree: &ExpressionTree{Expression: CompositeExpression{
				ExprType: AllOfExpression,
				SubExpressions: []Expression{
					FeatureExpression{Name: "juju"},
					FeatureExpression{Name: "holistic-uniter"},
				},
			}},
			feature: "holistic-uniter",
			expect:  true,
		},
		{
			about: "any of with optional feature",
			tree: &ExpressionTree{Expression: CompositeExpression{
				ExprType: AnyOfExpression,
				SubExpressions: []Expression{
					FeatureExpression{Name: "holistic-uniter"},
					FeatureExpression{Name: "juju"},
				},
			}},
			feature: "holistic-uniter",
		},
		{
			about: "any of requires feature in every branch",
			tree: &ExpressionTree{Expression: CompositeExpression{
				ExprType: AnyOfExpression,
				SubExpressions: []Expression{
					FeatureExpression{Name: "holistic-uniter"},
					CompositeExpression{
						ExprType: AllOfExpression,
						SubExpressions: []Expression{
							FeatureExpression{Name: "juju"},
							FeatureExpression{Name: "holistic-uniter"},
						},
					},
				},
			}},
			feature: "holistic-uniter",
			expect:  true,
		},
	}

	for _, test := range testCases {
		c.Check(RequiresFeature(test.tree, test.feature), tc.Equals, test.expect,
			tc.Commentf(test.about))
	}
}
