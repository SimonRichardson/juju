// Copyright 2026 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package assumes

// RequiresFeature reports whether every way to satisfy tree requires feature.
// A feature in an all-of expression is required when any subexpression requires
// it. A feature in an any-of expression is required only when every
// subexpression requires it.
func RequiresFeature(tree *ExpressionTree, feature string) bool {
	return tree != nil && requiresFeature(tree.Expression, feature)
}

func requiresFeature(expr Expression, feature string) bool {
	switch expr := expr.(type) {
	case FeatureExpression:
		return expr.Name == feature
	case *FeatureExpression:
		return expr != nil && expr.Name == feature
	case CompositeExpression:
		return compositeRequiresFeature(expr, feature)
	case *CompositeExpression:
		return expr != nil && compositeRequiresFeature(*expr, feature)
	default:
		return false
	}
}

func compositeRequiresFeature(expr CompositeExpression, feature string) bool {
	if expr.ExprType == AnyOfExpression {
		return allExpressionsRequireFeature(expr.SubExpressions, feature)
	}
	return anyExpressionRequiresFeature(expr.SubExpressions, feature)
}

func anyExpressionRequiresFeature(exprs []Expression, feature string) bool {
	for _, expr := range exprs {
		if requiresFeature(expr, feature) {
			return true
		}
	}
	return false
}

func allExpressionsRequireFeature(exprs []Expression, feature string) bool {
	for _, expr := range exprs {
		if !requiresFeature(expr, feature) {
			return false
		}
	}
	return len(exprs) > 0
}
