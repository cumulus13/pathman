package ui

import (
	"fmt"
	"strings"
)

// hexColor creates a color function from hex string using raw ANSI codes
func hexColor(hex string, bold bool) func(a ...interface{}) string {
	hex = strings.TrimPrefix(hex, "#")
	var r, g, b int
	fmt.Sscanf(hex, "%02x%02x%02x", &r, &g, &b)
	
	prefix := fmt.Sprintf("\033[38;2;%d;%d;%dm", r, g, b)
	if bold {
		prefix = fmt.Sprintf("\033[1;38;2;%d;%d;%dm", r, g, b)
	}
	
	return func(a ...interface{}) string {
		return prefix + fmt.Sprint(a...) + "\033[0m"
	}
}

var (
	// Primary colors (using hex codes)
	Info      = hexColor("#00FFFF", false)  // Cyan
	Success   = hexColor("#00FF00", true)   // Green Bold
	Warning   = hexColor("#FFD700", true)   // Gold Bold
	Error     = hexColor("#FF0000", true)   // Red Bold
	Highlight = hexColor("#FF69B4", true)   // Hot Pink Bold
	
	// Secondary styles
	Dim       = hexColor("#808080", false)  // Gray
	Path      = hexColor("#00FFFF", true)   // Blue Bold
	Title     = hexColor("#00FFFF", true)   // Cyan Bold
	
	// Headers
	HeaderInfo    = hexColor("#00CED1", true) // Dark Turquoise Bold
	HeaderSuccess = hexColor("#32CD32", true) // Lime Green Bold
	HeaderWarning = hexColor("#FFA500", true) // Orange Bold
	HeaderError   = hexColor("#DC143C", true) // Crimson Bold
	
	// Special formatters
	KeyValue = hexColor("#FFFF00", true)  // Gold
	Badge    = hexColor("#FF4500", true)   // Orange Red Bold
)