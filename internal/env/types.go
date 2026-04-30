package env

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strings"
)

// Scope defines where environment variables are stored
type Scope int

const (
	// ScopeUser represents user-level environment variables
	ScopeUser Scope = iota
	// ScopeSystem represents system-level environment variables (requires admin)
	ScopeSystem
)

func (s Scope) String() string {
	switch s {
	case ScopeUser:
		return "User"
	case ScopeSystem:
		return "System"
	default:
		return "Unknown"
	}
}

// Variable represents an environment variable
type Variable struct {
	Name  string
	Value string
	Scope Scope
}

// ValidationError represents an error during variable validation
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error [%s]: %s", e.Field, e.Message)
}

// PathEntry represents a single entry in a PATH-like variable
type PathEntry struct {
	Index int
	Value string
	Exist bool // Whether the path actually exists on disk
}

// VariableOutput represents a variable for output formatting
type VariableOutput struct {
	Name   string `json:"name" yaml:"name"`
	Value  string `json:"value" yaml:"value"`
	Scope  string `json:"scope" yaml:"scope"`
}

// VariablesOutput wraps multiple variables
type VariablesOutput struct {
	Total int              `json:"total" yaml:"total"`
	Vars  []VariableOutput `json:"variables" yaml:"variables"`
}

// ToJSON converts variables to JSON string
func (v *Variable) ToJSON() (string, error) {
	output := VariableOutput{
		Name:  v.Name,
		Value: v.Value,
		Scope: v.Scope.String(),
	}
	
	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %w", err)
	}
	return string(data), nil
}

// VariablesToJSON converts slice of variables to JSON
func VariablesToJSON(vars []Variable, scopeName string) (string, error) {
	output := VariablesOutput{
		Total: len(vars),
		Vars:  make([]VariableOutput, len(vars)),
	}
	
	for i, v := range vars {
		output.Vars[i] = VariableOutput{
			Name:  v.Name,
			Value: v.Value,
			Scope: v.Scope.String(),
		}
	}
	
	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %w", err)
	}
	return string(data), nil
}

// ToYAML converts variables to YAML string (simple implementation)
func (v *Variable) ToYAML() string {
	return fmt.Sprintf("name: %s\nvalue: %s\nscope: %s", v.Name, v.Value, v.Scope.String())
}

// VariablesToYAML converts slice of variables to YAML
func VariablesToYAML(vars []Variable) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("total: %d\nvariables:\n", len(vars)))
	
	for _, v := range vars {
		sb.WriteString(fmt.Sprintf("  - name: %s\n    value: %s\n    scope: %s\n", 
			v.Name, v.Value, v.Scope.String()))
	}
	
	return sb.String()
}

// ToCSV converts variables to CSV string
func VariablesToCSV(vars []Variable) (string, error) {
	var buf strings.Builder
	writer := csv.NewWriter(&buf)
	
	// Header
	writer.Write([]string{"Name", "Value", "Scope"})
	
	// Data
	for _, v := range vars {
		writer.Write([]string{v.Name, v.Value, v.Scope.String()})
	}
	
	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", fmt.Errorf("failed to write CSV: %w", err)
	}
	
	return buf.String(), nil
}

// FormatVariable outputs a variable in specified format
func FormatVariable(v *Variable, format string) (string, error) {
	switch strings.ToLower(format) {
	case "json":
		return v.ToJSON()
	case "yaml", "yml":
		return v.ToYAML(), nil
	case "csv":
		vars := []Variable{*v}
		return VariablesToCSV(vars)
	default:
		return fmt.Sprintf("%s=%s", v.Name, v.Value), nil
	}
}