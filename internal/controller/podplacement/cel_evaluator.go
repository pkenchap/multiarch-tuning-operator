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

package podplacement

import (
	"fmt"
	"sync"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/openshift/multiarch-tuning-operator/api/common/plugins"
)

// celEvaluator handles CEL expression compilation and evaluation for architecture placement rules
type celEvaluator struct {
	env           *cel.Env
	compiledRules map[string]cel.Program
	compileMutex  sync.RWMutex
}

// newCelEvaluator creates a new CEL evaluator with the Pod type registered
func newCelEvaluator() (*celEvaluator, error) {
	// Create CEL environment with Pod type
	env, err := cel.NewEnv(
		cel.Types(&corev1.Pod{}),
		cel.Variable("self", cel.ObjectType(schema.GroupVersionKind{
			Group:   "",
			Version: "v1",
			Kind:    "Pod",
		}.String())),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create CEL environment: %w", err)
	}

	return &celEvaluator{
		env:           env,
		compiledRules: make(map[string]cel.Program),
	}, nil
}

// compileRule compiles a CEL expression and caches it
func (e *celEvaluator) compileRule(ruleName, expression string) error {
	e.compileMutex.Lock()
	defer e.compileMutex.Unlock()

	// Check if already compiled
	if _, exists := e.compiledRules[ruleName]; exists {
		return nil
	}

	// Parse the expression
	ast, issues := e.env.Parse(expression)
	if issues != nil && issues.Err() != nil {
		return fmt.Errorf("failed to parse CEL expression for rule %s: %w", ruleName, issues.Err())
	}

	// Check the expression
	checked, issues := e.env.Check(ast)
	if issues != nil && issues.Err() != nil {
		return fmt.Errorf("failed to check CEL expression for rule %s: %w", ruleName, issues.Err())
	}

	// Verify the expression returns a boolean
	if checked.OutputType() != cel.BoolType {
		return fmt.Errorf("CEL expression for rule %s must return a boolean, got %s", ruleName, checked.OutputType())
	}

	// Compile the expression
	program, err := e.env.Program(checked)
	if err != nil {
		return fmt.Errorf("failed to compile CEL expression for rule %s: %w", ruleName, err)
	}

	e.compiledRules[ruleName] = program
	return nil
}

// evaluateRule evaluates a compiled CEL expression against a pod
func (e *celEvaluator) evaluateRule(ruleName string, pod *corev1.Pod) (bool, error) {
	e.compileMutex.RLock()
	program, exists := e.compiledRules[ruleName]
	e.compileMutex.RUnlock()

	if !exists {
		return false, fmt.Errorf("rule %s not compiled", ruleName)
	}

	// Evaluate the expression
	val, _, err := program.Eval(map[string]interface{}{
		"self": pod,
	})
	if err != nil {
		return false, fmt.Errorf("failed to evaluate CEL expression for rule %s: %w", ruleName, err)
	}

	// Convert to boolean
	boolVal, ok := val.(types.Bool)
	if !ok {
		return false, fmt.Errorf("CEL expression for rule %s did not return a boolean", ruleName)
	}

	return bool(boolVal), nil
}

// evaluateRules evaluates all rules in order and returns the first matching rule's architectures
// Returns the architectures and the name of the matching rule, or empty slice if no match
func (e *celEvaluator) evaluateRules(plugin *plugins.CelArchPlacement, pod *corev1.Pod) ([]string, string, error) {
	// Compile all rules first
	for _, rule := range plugin.Rules {
		if err := e.compileRule(rule.Name, rule.Expression); err != nil {
			return nil, "", err
		}
	}

	// Evaluate rules in order
	for _, rule := range plugin.Rules {
		matches, err := e.evaluateRule(rule.Name, pod)
		if err != nil {
			// Log error but continue to next rule
			return nil, "", fmt.Errorf("error evaluating rule %s: %w", rule.Name, err)
		}

		if matches {
			return rule.Architectures, rule.Name, nil
		}
	}

	// No rules matched, return default architectures
	return plugin.DefaultArchitectures, "", nil
}

// Global CEL evaluator instance (lazy initialized)
var (
	globalCelEvaluator     *celEvaluator
	globalCelEvaluatorOnce sync.Once
	globalCelEvaluatorErr  error
)

// getCelEvaluator returns the global CEL evaluator instance
func getCelEvaluator() (*celEvaluator, error) {
	globalCelEvaluatorOnce.Do(func() {
		globalCelEvaluator, globalCelEvaluatorErr = newCelEvaluator()
	})
	return globalCelEvaluator, globalCelEvaluatorErr
}
