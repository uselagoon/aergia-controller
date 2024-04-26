package idler

import (
	"k8s.io/apimachinery/pkg/labels"
)

func generateSelector(s idlerSelector) *labels.Requirement {
	r, _ := labels.NewRequirement(s.Name, s.Operator, s.Values)
	return r
}

func generateLabelRequirements(selectors []idlerSelector) []labels.Requirement {
	labelRequirements := []labels.Requirement{}
	for _, rs := range selectors {
		selector := generateSelector(rs)
		labelRequirements = append(labelRequirements, *selector)
	}
	return labelRequirements
}
