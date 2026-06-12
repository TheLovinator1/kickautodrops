package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// Color palette
const (
	colorHeader    = "#7C3AED" // purple
	colorSubtext   = "#94A3B8" // muted grey
	colorSuccess   = "#22C55E" // green
	colorWarning   = "#EAB308" // yellow
	colorError     = "#EF4444" // red
	colorInfo      = "#06B6D4" // cyan
	colorAccent    = "#3B82F6" // blue
	colorClaimed   = "#22C55E" // green for claimed
	colorUnclaimed = "#F59E0B" // amber for in-progress
)

// Layout styles
var (
	// App wrapper — single RoundedBorder around the entire TUI
	AppStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(colorAccent))

	// Header
	HeaderStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorHeader)).
			Bold(true).
			Padding(0, 0)

	HeaderSubStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorSubtext))

	// Log panel — no border, just text foreground
	LogPanelStyle = lipgloss.NewStyle()

	// Tree panel — no border
	TreePanelStyle = lipgloss.NewStyle()

	TreeGameStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorAccent)).
			Bold(true)

	TreeCampaignStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorInfo))

	TreeRewardClaimedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorClaimed))

	TreeRewardProgressStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorUnclaimed))

	// Status bar / footer — no border
	FooterStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorSubtext))

	FooterTimerStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorAccent)).
				Bold(true)

	FooterStreamerStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorSuccess))

	FooterStatusStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorInfo))

	// Help text
	HelpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorSubtext)).
			Italic(true)
)
