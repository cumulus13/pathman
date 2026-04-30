package ui

// Icons for different status messages
const (
	IconSuccess = "✅"
	IconError   = "❌"
	IconWarning = "⚠️ "
	IconInfo    = "ℹ️ "
	IconStar    = "⭐"
	IconFolder  = "📁"
	IconGear    = "⚙️ "
	IconUser    = "👤"
	IconSystem  = "🖥️ "
	IconPlus    = "➕"
	IconMinus   = "➖"
	IconSearch  = "🔍"
	IconRocket  = "🚀"
	IconLock    = "🔒"
	IconKey     = "🔑"
	IconSave    = "💾"
	IconDelete  = "🗑️ "
	IconList    = "📋"
	IconRefresh = "🔄"
	IconCheck   = "✔️ "
	IconCross   = "✖️ "
	IconLink    = "🔗"
	IconBroken  = "💔"
)

// GetScopeIcon returns appropriate icon for scope
func GetScopeIcon(scope string) string {
	switch scope {
	case "user":
		return IconUser
	case "system":
		return IconSystem
	default:
		return IconGear
	}
}