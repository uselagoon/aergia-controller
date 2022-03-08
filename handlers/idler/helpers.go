package idler

import (
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/labels"
)

func generateSelector(s idlerSelector) *labels.Requirement {
	r, _ := labels.NewRequirement(s.Name, s.Operator, s.Values)
	return r
}

func yamlGenerateLabelRequirements(selectors []byte) []labels.Requirement {
	labelRequirements := []labels.Requirement{}
	convertedYaml := []idlerSelector{}
	_ = yaml.Unmarshal([]byte(selectors), &convertedYaml)
	for _, rs := range convertedYaml {
		selector := generateSelector(rs)
		labelRequirements = append(labelRequirements, *selector)
	}
	return labelRequirements
}

func generateLabelRequirements(selectors []idlerSelector) []labels.Requirement {
	labelRequirements := []labels.Requirement{}
	for _, rs := range selectors {
		selector := generateSelector(rs)
		labelRequirements = append(labelRequirements, *selector)
	}
	return labelRequirements
}

func yamlToIdler(selectors []byte) IdlerData {
	convertedYaml := IdlerData{}
	_ = yaml.Unmarshal([]byte(selectors), &convertedYaml)
	return convertedYaml
}
