/*
Copyright 2025 Red Hat, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package plugins

import "fmt"

const (
	// CelArchPlacementPluginName stores the name for the celArchPlacement plugin.
	CelArchPlacementPluginName = "CelArchPlacement"
)

// CelArchPlacement is a plugin that provides CEL-based architecture selection rules.
// This plugin is only available in namespace-scoped PodPlacementConfig resources.
// When a rule matches, the plugin removes any existing architecture constraints from the pod's
// nodeSelector and nodeAffinity, then sets new architecture constraints based on the rule.
type CelArchPlacement struct {
	BasePlugin `json:",inline"`

	// DefaultArchitectures is a required list of architectures to use when no rules match.
	// This limits the explosion of possible rules by providing a sensible default.
	// When applied, existing architecture constraints are removed and replaced with these architectures.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=4
	DefaultArchitectures []string `json:"defaultArchitectures" protobuf:"bytes,2,rep,name=defaultArchitectures"`

	// Rules is a list of architecture selection rules evaluated in order.
	// The first matching rule determines the target architecture.
	// When a rule matches, existing architecture constraints are removed and replaced.
	// +optional
	// +kubebuilder:validation:MaxItems=500
	Rules []ArchitectureRule `json:"rules,omitempty" protobuf:"bytes,3,rep,name=rules"`
}

// ArchitectureRule defines a single CEL-based rule for architecture selection
type ArchitectureRule struct {
	// Name is a descriptive name for this rule
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	Name string `json:"name" protobuf:"bytes,1,opt,name=name"`

	// Expression is a CEL expression that evaluates against a Pod resource.
	// The expression must return a boolean value.
	// The expression has access to the pod via the 'self' variable.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Expression string `json:"expression" protobuf:"bytes,2,opt,name=expression"`

	// Architectures is the list of target architectures to use when this rule matches.
	// When applied, any existing architecture constraints in the pod's nodeSelector
	// and nodeAffinity are removed and replaced with these architectures.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=4
	Architectures []string `json:"architectures" protobuf:"bytes,3,rep,name=architectures"`
}

// Name returns the name of the CelArchPlacementPluginName.
func (c *CelArchPlacement) Name() string {
	return CelArchPlacementPluginName
}

// ValidateArchitectures checks whether the architectures are valid
func (c *CelArchPlacement) ValidateArchitectures() error {
	validArchs := map[string]bool{
		"amd64": true, "arm64": true, "ppc64le": true, "s390x": true,
	}

	// Validate default architectures
	for _, arch := range c.DefaultArchitectures {
		if !validArchs[arch] {
			return fmt.Errorf("invalid default architecture: %s", arch)
		}
	}

	// Validate rule architectures
	for _, rule := range c.Rules {
		for _, arch := range rule.Architectures {
			if !validArchs[arch] {
				return fmt.Errorf("invalid architecture in rule %s: %s", rule.Name, arch)
			}
		}
	}

	return nil
}

// ValidateRuleNames checks whether rule names are unique
func (c *CelArchPlacement) ValidateRuleNames() error {
	seen := make(map[string]bool)
	for _, rule := range c.Rules {
		if seen[rule.Name] {
			return fmt.Errorf("duplicate rule name: %s", rule.Name)
		}
		seen[rule.Name] = true
	}
	return nil
}
