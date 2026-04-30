package ui

import "github.com/fatih/color"

var (
	// Primary colors
	Info      = color.New(color.FgCyan).SprintFunc()
	Success   = color.New(color.FgGreen, color.Bold).SprintFunc()
	Warning   = color.New(color.FgYellow, color.Bold).SprintFunc()
	Error     = color.New(color.FgRed, color.Bold).SprintFunc()
	Highlight = color.New(color.FgMagenta, color.Bold).SprintFunc()
	
	// Secondary styles
	Dim       = color.New(color.FgWhite, color.Faint).SprintFunc()
	Path      = color.New(color.FgBlue).SprintFunc()
	Title     = color.New(color.FgCyan, color.Bold).SprintFunc()
	
	// Headers
	HeaderInfo    = color.New(color.FgCyan, color.Bold, color.Underline).SprintFunc()
	HeaderSuccess = color.New(color.FgGreen, color.Bold, color.Underline).SprintFunc()
	HeaderWarning = color.New(color.FgYellow, color.Bold, color.Underline).SprintFunc()
	HeaderError   = color.New(color.FgRed, color.Bold, color.Underline).SprintFunc()
)

// Special formatters
var (
	KeyValue = color.New(color.FgYellow).SprintFunc()
	Badge    = color.New(color.BgCyan, color.FgBlack, color.Bold).SprintFunc()
)