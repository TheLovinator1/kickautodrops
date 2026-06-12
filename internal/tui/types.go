package tui

// TreeNode represents a node in the drops tree display.
type TreeNode struct {
	Label    string // display text
	Icon     string // optional emoji/icon prefix
	Info     string // extra info like "60%", "Claimed"
	Children []*TreeNode
	Expanded bool
	Depth    int // 0=game, 1=campaign, 2=reward
}

// ---- Bubbletea message types ----

// LogMsg carries a single log line from worker goroutines.
type LogMsg string

// TreeUpdateMsg carries an updated drops tree for the right panel.
type TreeUpdateMsg struct {
	Nodes []*TreeNode
}

// StatusUpdateMsg updates the status bar.
type StatusUpdateMsg struct {
	Streamer string
	Status   string
	Elapsed  string // optional pre-formatted elapsed string
}

// TimerTickMsg is sent every second to update the elapsed timer.
type TimerTickMsg struct{}

// RemainingTimeMsg carries the total estimated time remaining for all drops.
type RemainingTimeMsg struct {
	Seconds int // total seconds of watch time remaining
}

// ErrorMsg is sent when an unrecoverable error occurs.
type ErrorMsg struct {
	Err error
}
