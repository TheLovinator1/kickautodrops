package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Model is the main TUI state.
type Model struct {
	// Log panel
	logLines    []string
	logViewport viewport.Model
	logReady    bool

	// Tree panel (drops)
	treeData []*TreeNode

	// Timer
	startTime      time.Time
	elapsed        time.Duration
	totalRemaining time.Duration // estimated total watch time remaining

	// Status
	currentStreamer string
	statusText      string

	// Layout
	width  int
	height int

	// Channel for receiving log lines from worker goroutines
	LogCh chan string

	// Quit flag
	quitting bool
}

// NewModel creates a new TUI model with the given log channel capacity.
func NewModel(logCh chan string) Model {
	return Model{
		logLines:        make([]string, 0, 1000),
		startTime:       time.Now(),
		currentStreamer: "Idle",
		statusText:      "Initializing...",
		LogCh:           logCh,
	}
}

// Init initialises the bubbletea program.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tea.EnterAltScreen,
		m.listenForLogs(),
		tickEverySecond(),
	)
}

// listenForLogs returns a command that reads from the log channel.
func (m Model) listenForLogs() tea.Cmd {
	return func() tea.Msg {
		line, ok := <-m.LogCh
		if !ok {
			return nil
		}
		return LogMsg(line)
	}
}

// tickEverySecond returns a command that sends a timer tick every second.
func tickEverySecond() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return TimerTickMsg{}
	})
}

// Update handles all incoming messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		if !m.logReady {
			m.logViewport = viewport.New(1, 1)
			m.logViewport.YPosition = 0
			m.logViewport.HighPerformanceRendering = false
			m.logReady = true
		}

		// Outer RoundedBorder takes 2 cols/rows.
		innerW := m.width - 2
		innerH := m.height - 2
		if innerW < 20 {
			innerW = 20
		}
		if innerH < 3 {
			innerH = 3
		}

		logInnerW := innerW * 55 / 100
		if logInnerW < 10 {
			logInnerW = 10
		}
		contentH := innerH - 1 - 1 // header + footer

		m.logViewport.Width = logInnerW
		m.logViewport.Height = contentH
		m.logViewport.GotoBottom()

		return m, nil

	case LogMsg:
		line := string(msg)
		m.logLines = append(m.logLines, line)
		// Cap at 1000 lines
		if len(m.logLines) > 1000 {
			m.logLines = m.logLines[len(m.logLines)-1000:]
		}
		m.logViewport.SetContent(strings.Join(m.logLines, "\n"))
		m.logViewport.GotoBottom()
		// Re-queue the log listener
		return m, m.listenForLogs()

	case TreeUpdateMsg:
		m.treeData = msg.Nodes
		return m, nil

	case StatusUpdateMsg:
		if msg.Streamer != "" {
			m.currentStreamer = msg.Streamer
		}
		if msg.Status != "" {
			m.statusText = msg.Status
		}
		return m, nil

	case TimerTickMsg:
		if !m.quitting {
			m.elapsed = time.Since(m.startTime)
			cmds = append(cmds, tickEverySecond())
		}
		return m, tea.Batch(cmds...)

	case RemainingTimeMsg:
		m.totalRemaining = time.Duration(msg.Seconds) * time.Second
		return m, nil

	case ErrorMsg:
		m.logLines = append(m.logLines, fmt.Sprintf("❌ Error: %v", msg.Err))
		m.logViewport.SetContent(strings.Join(m.logLines, "\n"))
		m.logViewport.GotoBottom()
		return m, m.listenForLogs()

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit
		case "ctrl+s":
			saved := saveLogs(m.logLines)
			m.logLines = append(m.logLines, saved)
			m.logViewport.SetContent(strings.Join(m.logLines, "\n"))
			m.logViewport.GotoBottom()
		case "up":
			m.logViewport.LineUp(1)
		case "down":
			m.logViewport.LineDown(1)
		case "pgup":
			m.logViewport.HalfViewUp()
		case "pgdown":
			m.logViewport.HalfViewDown()
		}
		return m, nil
	}

	return m, tea.Batch(cmds...)
}

// View renders the complete TUI.
func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	// Outer RoundedBorder takes 2 cols and 2 rows.
	innerW := m.width - 2
	innerH := m.height - 2
	if innerW < 20 || innerH < 3 {
		return "Terminal too small"
	}

	logW := innerW * 55 / 100
	if logW < 10 {
		logW = 10
	}
	treeW := innerW - logW - 1 // 1-col separator
	if treeW < 5 {
		treeW = 5
	}

	headerH := 1
	contentH := innerH - headerH - 1

	// ---- Header ----
	header := lipgloss.JoinHorizontal(
		lipgloss.Center,
		HeaderStyle.Render("🎮 KickAutoDrops"),
		HeaderSubStyle.Render(fmt.Sprintf("  %s", time.Now().Format("15:04:05"))),
	)
	header = lipgloss.NewStyle().Width(innerW).Render(header)

	// ---- Log panel ----
	logLines := m.logViewport.View()
	logLines = lipgloss.NewStyle().Width(logW).Render(logLines)
	logLines = padHeight(logLines, contentH)
	logPanel := LogPanelStyle.Render(logLines)

	// ---- Tree panel ----
	treeLines := renderTree(m.treeData, treeW)
	treeLines = lipgloss.NewStyle().Width(treeW).Render(treeLines)
	treeLines = padHeight(treeLines, contentH)
	treePanel := TreePanelStyle.Render(treeLines)

	// ---- Content (log | tree) ----
	sep := lipgloss.NewStyle().
		Foreground(lipgloss.Color(colorSubtext)).
		Render("│")
	content := lipgloss.JoinHorizontal(
		lipgloss.Top,
		logPanel,
		sep,
		treePanel,
	)
	content = lipgloss.NewStyle().Width(innerW).Render(content)

	// ---- Footer ----
	timerStr := formatDuration(m.elapsed)
	if m.totalRemaining > 0 {
		timerStr = fmt.Sprintf("%s / %s", timerStr, formatDuration(m.totalRemaining))
	}
	footerText := fmt.Sprintf("⏱ %s  │  👤 %s  │  📡 %s  │  Ctrl+S save logs to disk",
		timerStr, m.currentStreamer, m.statusText)
	footer := FooterStyle.Width(innerW).Render(footerText)

	// ---- Full layout inside outer border ----
	body := lipgloss.JoinVertical(lipgloss.Left, header, content, footer)
	return AppStyle.Render(body)
}

// ---- Helpers ----

// saveLogs writes all log lines to a timestamped file and returns a status message.
func saveLogs(lines []string) string {
	ts := time.Now().Format("2006-01-02_15-04-05")
	name := fmt.Sprintf("kickautodrops_logs_%s.txt", ts)
	path := name
	// Try to use a sensible default location
	if dir, err := os.Getwd(); err == nil {
		path = filepath.Join(dir, name)
	}
	data := strings.Join(lines, "\n")
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		return fmt.Sprintf("❌ Failed to save logs: %v", err)
	}
	return fmt.Sprintf("💾 Logs saved to %s (%d lines)", path, len(lines))
}

// padHeight ensures s is exactly h lines tall by appending empty lines.
func padHeight(s string, h int) string {
	lines := strings.Split(s, "\n")
	for len(lines) < h {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

// ---- Tree rendering ----

// renderTree renders the tree nodes as a string.
func renderTree(nodes []*TreeNode, maxWidth int) string {
	if len(nodes) == 0 {
		return HelpStyle.Render("No drops data yet.\nWaiting for campaigns...")
	}

	var b strings.Builder
	for _, node := range nodes {
		renderNode(&b, node, "", true, maxWidth)
	}
	return b.String()
}

// renderNode recursively renders a tree node with proper indentation.
func renderNode(b *strings.Builder, node *TreeNode, prefix string, isLast bool, maxWidth int) {
	// Build the connector
	connector := "├── "
	childPrefix := "│   "
	if isLast {
		connector = "└── "
		childPrefix = "    "
	}

	// Render the label with appropriate styling
	var line string
	icon := node.Icon
	if icon == "" {
		icon = "  "
	}

	switch node.Depth {
	case 0: // Game level
		label := lipgloss.NewStyle().Width(maxWidth - len(prefix) - 4).Render(node.Label)
		line = fmt.Sprintf("%s%s%s", prefix, connector, TreeGameStyle.Render(fmt.Sprintf("%s %s", icon, label)))
	case 1: // Campaign level
		label := lipgloss.NewStyle().Width(maxWidth - len(prefix) - 4).Render(node.Label)
		line = fmt.Sprintf("%s%s%s", prefix, connector, TreeCampaignStyle.Render(fmt.Sprintf("%s %s", icon, label)))
	case 2: // Reward level
		statusIcon := "○"
		statusStyle := TreeRewardProgressStyle
		if node.Icon == "✓" || strings.Contains(node.Label, "✓") {
			statusIcon = "✓"
			statusStyle = TreeRewardClaimedStyle
		}
		label := node.Label
		if node.Info != "" {
			label = fmt.Sprintf("%s [%s]", node.Label, node.Info)
		}
		line = fmt.Sprintf("%s%s%s", prefix, connector, statusStyle.Render(fmt.Sprintf("%s %s", statusIcon, label)))
	}

	b.WriteString(line)
	b.WriteString("\n")

	// Render children
	for i, child := range node.Children {
		renderNode(b, child, prefix+childPrefix, i == len(node.Children)-1, maxWidth)
	}
}

// ---- Helpers ----

// formatDuration formats a duration as MM:SS.
func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}

// BuildTreeFromRewards builds a tree from drops reward data.
// rewards is grouped by CategoryID/CampaignID.
func BuildTreeFromRewards(
	rewards []RewardInfo,
	knownCategories map[int]string, // categoryID -> category name
) []*TreeNode {
	type campGroup struct {
		ID    string
		Name  string
		Items []RewardInfo
	}

	// Group by category
	catGroups := make(map[int][]campGroup)
	catMap := make(map[int]map[string][]RewardInfo) // categoryID -> campaignID -> rewards

	for _, r := range rewards {
		if catMap[r.CategoryID] == nil {
			catMap[r.CategoryID] = make(map[string][]RewardInfo)
		}
		catMap[r.CategoryID][r.CampaignID] = append(catMap[r.CategoryID][r.CampaignID], r)
	}

	for catID, campaigns := range catMap {
		var groups []campGroup
		for campID, items := range campaigns {
			campName := items[0].CampaignName
			if campName == "" {
				campName = campID
			}
			groups = append(groups, campGroup{ID: campID, Name: campName, Items: items})
		}
		catGroups[catID] = groups
	}

	// Sort category IDs by name
	var catIDs []int
	for catID := range catGroups {
		catIDs = append(catIDs, catID)
	}
	sort.Slice(catIDs, func(i, j int) bool {
		ni := knownCategories[catIDs[i]]
		if ni == "" {
			ni = fmt.Sprintf("Category %d", catIDs[i])
		}
		nj := knownCategories[catIDs[j]]
		if nj == "" {
			nj = fmt.Sprintf("Category %d", catIDs[j])
		}
		return ni < nj
	})

	var roots []*TreeNode

	for _, catID := range catIDs {
		campaigns := catGroups[catID]
		catName := knownCategories[catID]
		if catName == "" {
			catName = fmt.Sprintf("Category %d", catID)
		}

		// Sort campaigns by name
		sort.Slice(campaigns, func(i, j int) bool {
			return campaigns[i].Name < campaigns[j].Name
		})

		gameNode := &TreeNode{
			Label:    catName,
			Icon:     "🎮",
			Expanded: true,
			Depth:    0,
		}

		for _, camp := range campaigns {
			// Sort rewards by name
			sort.Slice(camp.Items, func(i, j int) bool {
				return camp.Items[i].Name < camp.Items[j].Name
			})

			campNode := &TreeNode{
				Label:    camp.Name,
				Icon:     "📦",
				Expanded: true,
				Depth:    1,
			}

			for _, r := range camp.Items {
				var rewardIcon string
				var info string
				if r.Claimed {
					rewardIcon = "✓"
					info = "Claimed"
				} else if r.RequiredUnits > 0 {
					pct := (1.0 - r.RemainingUnits/r.RequiredUnits) * 100
					if pct < 0 {
						pct = 0
					}
					info = fmt.Sprintf("%.0f%%", pct)
				} else {
					rewardIcon = "○"
				}

				rewardNode := &TreeNode{
					Label:    r.Name,
					Icon:     rewardIcon,
					Info:     info,
					Expanded: true,
					Depth:    2,
				}
				campNode.Children = append(campNode.Children, rewardNode)
			}

			gameNode.Children = append(gameNode.Children, campNode)
		}

		roots = append(roots, gameNode)
	}

	return roots
}

// RewardInfo is a simplified reward view for tree building.
type RewardInfo struct {
	ID             string
	CampaignID     string
	CampaignName   string
	Name           string
	RequiredUnits  float64
	RemainingUnits float64
	Claimed        bool
	CategoryID     int
	CategoryName   string
}
