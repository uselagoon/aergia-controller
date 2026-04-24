package idler

import (
	"strings"

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

func addStatusCode(codes string, code string) *string {
	if codes == "" {
		return &code
	}
	parts := strings.Split(codes, ",")
	for _, c := range parts {
		if c == code {
			return &codes
		}
	}
	newCodes := codes + "," + code
	return &newCodes
}
