package env

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strings"
)

type Scope int

const (
	ScopeUser Scope = iota
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

type AppendPosition int

const (
	AppendPost AppendPosition = iota
	AppendPre
)

func (p AppendPosition) String() string {
	switch p {
	case AppendPost:
		return "post"
	case AppendPre:
		return "pre"
	default:
		return "unknown"
	}
}

type Variable struct {
	Name  string
	Value string
	Scope Scope
}

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error [%s]: %s", e.Field, e.Message)
}

type PathEntry struct {
	Index int
	Value string
	Exist bool
}

type VariableOutput struct {
	Name  string `json:"name"`
	Value string `json:"value"`
	Scope string `json:"scope"`
}

type VariablesOutput struct {
	Total int              `json:"total"`
	Vars  []VariableOutput `json:"variables"`
}

func NormalizePathSeparator(value string) string {
	for strings.Contains(value, ";;") {
		value = strings.ReplaceAll(value, ";;", ";")
	}
	return strings.Trim(value, ";")
}

func VariablesToJSON(vars []Variable) (string, error) {
	output := VariablesOutput{Total: len(vars), Vars: make([]VariableOutput, len(vars))}
	for i, v := range vars {
		output.Vars[i] = VariableOutput{Name: v.Name, Value: v.Value, Scope: v.Scope.String()}
	}
	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %w", err)
	}
	return string(data), nil
}

func VariablesToYAML(vars []Variable) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("total: %d\nvariables:\n", len(vars)))
	for _, v := range vars {
		sb.WriteString(fmt.Sprintf("  - name: %s\n    value: \"%s\"\n    scope: %s\n", v.Name, v.Value, v.Scope.String()))
	}
	return sb.String()
}

func VariablesToCSV(vars []Variable) (string, error) {
	var buf strings.Builder
	writer := csv.NewWriter(&buf)
	writer.Write([]string{"Name", "Value", "Scope"})
	for _, v := range vars {
		writer.Write([]string{v.Name, v.Value, v.Scope.String()})
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", fmt.Errorf("failed to write CSV: %w", err)
	}
	return buf.String(), nil
}

func VariablesToTable(vars []Variable) string {
	var sb strings.Builder
	maxLen := 22
	for _, v := range vars {
		if len(v.Name) > maxLen {
			maxLen = len(v.Name)
		}
	}
	maxLen += 3
	sb.WriteString(fmt.Sprintf("%-*s %-60s %s\n", maxLen, "NAME", "VALUE", "SCOPE"))
	sb.WriteString(strings.Repeat("─", maxLen+66) + "\n")
	for _, v := range vars {
		value := v.Value
		if len(value) > 60 {
			value = value[:57] + "..."
		}
		sb.WriteString(fmt.Sprintf("%-*s %-60s %s\n", maxLen, v.Name, value, v.Scope.String()))
	}
	return sb.String()
}

func FormatVariables(vars []Variable, format string) (string, error) {
	switch strings.ToLower(format) {
	case "json":
		return VariablesToJSON(vars)
	case "yaml", "yml":
		return VariablesToYAML(vars), nil
	case "csv":
		return VariablesToCSV(vars)
	case "table":
		return VariablesToTable(vars), nil
	default:
		var sb strings.Builder
		for _, v := range vars {
			sb.WriteString(fmt.Sprintf("%s=%s [%s]\n", v.Name, v.Value, v.Scope.String()))
		}
		return sb.String(), nil
	}
}

func FormatVariable(v *Variable, format string) (string, error) {
	return FormatVariables([]Variable{*v}, format)
}