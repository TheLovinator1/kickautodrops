package tui

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"kickautodrops/internal/cookies"
	"kickautodrops/internal/drops"
	"kickautodrops/internal/kick"
	"kickautodrops/internal/log"
	"kickautodrops/internal/viewer"
)

// programRef holds a reference to the running bubbletea program,
// set by RunFarming so the farming loop can send messages to the TUI.
var programRef *tea.Program

// FarmingMode represents the type of drop farming to do.
type FarmingMode int

const (
	ModeStreamer FarmingMode = iota // watch specific streamers from campaigns
	ModeGeneral                     // watch random category streamers
)

// RunFarming starts the farming loop in a background goroutine.
// It stores the program reference for sending tree updates to the TUI.
func RunFarming(
	ctx context.Context,
	client *kick.Client,
	logCh chan string,
	program *tea.Program,
	mode FarmingMode,
	categoryID int,
) {
	programRef = program
	go func() {
		if err := farmingLoop(ctx, client, logCh, mode, categoryID); err != nil {
			select {
			case logCh <- fmt.Sprintf("❌ Fatal error: %v", err):
			default:
			}
		}
	}()
}

// farmingLoop is the main orchestration loop.
func farmingLoop(
	ctx context.Context,
	client *kick.Client,
	logCh chan string,
	mode FarmingMode,
	categoryID int,
) error {

	logCh <- "🚀 KickAutoDrops starting..."
	logCh <- ""
	sendStatusUpdate("", "Starting...")

	// ---- Step 1: Load cookies ----
	logCh <- "📋 Loading cookies..."
	cookieMap, err := cookies.Load("cookies.txt")
	if err != nil {
		logCh <- ""
		logCh <- "❌ cookies.txt not found!"
		logCh <- ""
		logCh <- "📌 How to set up cookies.txt:"
		logCh <- ""
		logCh <- "  1. Install a cookie export extension:"
		logCh <- "     Chrome:  https://chromewebstore.google.com/detail/get-cookiestxt-locally/cclelndahbckbenkjhflpdbgdldlbecc"
		logCh <- "     Firefox: https://addons.mozilla.org/en-US/firefox/addon/get-cookies-txt-locally/"
		logCh <- ""
		logCh <- "  2. Go to https://kick.com and log in"
		logCh <- "  3. Click the extension → Export all cookies"
		logCh <- "  4. Place the exported cookies.txt next to the executable"
		wd, _ := os.Getwd()
		logCh <- fmt.Sprintf("     Expected path: %s/cookies.txt", wd)
		return fmt.Errorf("cookies.txt not found")
	}
	logCh <- fmt.Sprintf("📋 ✓ Loaded %d cookies", len(cookieMap))

	// ---- Step 2: Fetch campaigns ----
	logCh <- ""
	logCh <- "📡 Fetching drop campaigns..."
	campaigns, err := client.GetAllCampaigns()
	if err != nil {
		return fmt.Errorf("get campaigns: %w", err)
	}
	logCh <- fmt.Sprintf("📡 ✓ Got %d campaigns", len(campaigns.Data))

	// ---- Step 3: Create manager ----
	mgr := drops.NewManager(campaigns)

	// Build category name map from campaigns response
	categoryNames := make(map[int]string)
	for _, c := range campaigns.Data {
		if c.Category.ID != 0 {
			if name := c.Category.Name; name != "" {
				categoryNames[c.Category.ID] = name
			}
		}
	}

	// ---- Step 4: Sync/claim initial state (using cached cookies) ----
	logCh <- ""
	logCh <- "🔄 Checking claim status..."
	if err := syncWithCookies(client, mgr, cookieMap); err != nil {
		logCh <- fmt.Sprintf("⚠ Sync warning: %v", err)
	} else {
		logCh <- "🔄 ✓ Claim check complete"
	}

	// ---- Step 5: Build and send initial tree ----
	sendRemainingTime(mgr)
	tree := buildTreeFromManager(campaigns, mgr, categoryNames)
	sendTreeUpdate(logCh, tree)

	// ---- Step 6: Start periodic tree refresh (every 120s, no disk I/O) ----
	go func() {
		ticker := time.NewTicker(120 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := syncWithCookies(client, mgr, cookieMap); err == nil {
					sendRemainingTime(mgr)
					tree := buildTreeFromManager(campaigns, mgr, categoryNames)
					sendTreeUpdate(logCh, tree)
				}
			}
		}
	}()

	// ---- Step 7: Run the farming loop based on mode ----
	switch mode {
	case ModeStreamer:
		return runStreamerMode(ctx, client, mgr, logCh, cookieMap, categoryID, categoryNames)
	case ModeGeneral:
		return runGeneralMode(ctx, client, mgr, logCh, cookieMap, categoryID, categoryNames)
	default:
		return runGeneralMode(ctx, client, mgr, logCh, cookieMap, categoryID, categoryNames)
	}
}

// runStreamerMode watches specific streamers from campaigns.
func runStreamerMode(
	ctx context.Context,
	client *kick.Client,
	mgr *drops.Manager,
	logCh chan string,
	cookieMap map[string]string,
	categoryID int,
	categoryNames map[int]string,
) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		targets := mgr.CollectUsernames()
		if len(targets) == 0 {
			logCh <- "✅ All streamer drops completed! Falling back to general drops..."
			return runGeneralMode(ctx, client, mgr, logCh, cookieMap, categoryID, categoryNames)
		}

		for _, target := range targets {
			if target.Claimed {
				continue
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			logCh <- ""
			logCh <- fmt.Sprintf("🔍 Checking %s...", target.Username)
			sendStatusUpdate("", fmt.Sprintf("🔍 Checking %s...", target.Username))
			// Check if online and playing the right game
			info, err := client.GetStreamInfo(target.Username)
			if err != nil {
				logCh <- fmt.Sprintf("⚠ Error checking %s: %v", target.Username, err)
				continue
			}

			if !info.IsLive {
				logCh <- fmt.Sprintf("✗ %s is offline", target.Username)
				sendStatusUpdate("", fmt.Sprintf("✗ %s offline", target.Username))
				continue
			}

			if info.GameID() != 0 && info.GameID() != categoryID {
				logCh <- fmt.Sprintf("✗ %s is playing another game (category %d)", target.Username, info.GameID())
				sendStatusUpdate("", fmt.Sprintf("✗ %s wrong game", target.Username))
				continue
			}

			// Watch this streamer
			remaining := target.RequiredSeconds
			watchDuration := time.Duration(remaining+120) * time.Second // +2 min buffer
			logCh <- fmt.Sprintf("⏱ Watching %s for %v...", target.Username, watchDuration)
			sendStatusUpdate(target.Username, "Watching...")

			// Rebuild tree with watching status
			tree := buildTreeFromManager(nil, mgr, categoryNames)
			sendTreeUpdate(logCh, tree)

			earlyEnd, err := viewer.RunWithTimer(ctx, client, mgr, target.Username, categoryID, watchDuration, cookieMap)
			if err != nil {
				logCh <- fmt.Sprintf("❌ Error watching %s: %v", target.Username, err)
				sendStatusUpdate("", "Error")
				time.Sleep(10 * time.Second)
				continue
			}

			if earlyEnd {
				logCh <- fmt.Sprintf("⚠ %s stream ended early, finding next...", target.Username)
				sendStatusUpdate("", "Stream ended early")
				time.Sleep(5 * time.Second)
				break // re-collect usernames
			}

			logCh <- fmt.Sprintf("✅ Finished watching %s", target.Username)
			sendStatusUpdate("", "Done, syncing...")

			// Sync progress (no disk I/O, uses cached cookies)
			if err := syncWithCookies(client, mgr, cookieMap); err != nil {
				logCh <- fmt.Sprintf("⚠ Sync warning: %v", err)
			}
			sendRemainingTime(mgr)

			// Rebuild tree
			tree = buildTreeFromManager(nil, mgr, categoryNames)
			sendTreeUpdate(logCh, tree)
		}

		// Wait before checking again
		logCh <- "⏳ Waiting 30s before next check..."
		sendStatusUpdate("", "Waiting 30s...")
		time.Sleep(30 * time.Second)
	}
}

// runGeneralMode watches random streamers from a category.
func runGeneralMode(
	ctx context.Context,
	client *kick.Client,
	mgr *drops.Manager,
	logCh chan string,
	cookieMap map[string]string,
	categoryID int,
	categoryNames map[int]string,
) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Check if any general drops are still needed
		remaining := mgr.GetRemainingTime("") // empty = fallback to general
		if remaining <= 0 {
			logCh <- "✅ General drops completed! Checking for more..."
			if err := syncWithCookies(client, mgr, cookieMap); err != nil {
				logCh <- fmt.Sprintf("⚠ Sync warning: %v", err)
			}
			sendRemainingTime(mgr)
			tree := buildTreeFromManager(nil, mgr, categoryNames)
			sendTreeUpdate(logCh, tree)
			time.Sleep(60 * time.Second)
			continue
		}

		// Find a random streamer
		stream, err := client.GetRandomStreamFromCategory(categoryID, 10)
		if err != nil {
			logCh <- fmt.Sprintf("⚠ No streams found in category %d, retrying in 30s...", categoryID)
			time.Sleep(30 * time.Second)
			continue
		}

		username := stream.Channel.Username
		logCh <- fmt.Sprintf("🎲 Selected random streamer: %s", username)

		watchDuration := time.Duration(remaining+120) * time.Second
		logCh <- fmt.Sprintf("⏱ Watching %s for %v...", username, watchDuration)
		sendStatusUpdate(username, "Watching...")

		earlyEnd, err := viewer.RunWithTimer(ctx, client, mgr, username, categoryID, watchDuration, cookieMap)
		if err != nil {
			logCh <- fmt.Sprintf("❌ Error watching %s: %v", username, err)
			sendStatusUpdate("", "Error")
			time.Sleep(10 * time.Second)
			continue
		}

		if earlyEnd {
			logCh <- fmt.Sprintf("⚠ %s stream ended early", username)
			sendStatusUpdate("", "Stream ended early")
			time.Sleep(5 * time.Second)
			continue
		}

		logCh <- fmt.Sprintf("✅ Finished watching %s", username)
		sendStatusUpdate("", "Syncing...")

		// Sync and update tree
		if err := syncWithCookies(client, mgr, cookieMap); err != nil {
			logCh <- fmt.Sprintf("⚠ Sync warning: %v", err)
		}
		sendRemainingTime(mgr)
		tree := buildTreeFromManager(nil, mgr, categoryNames)
		sendTreeUpdate(logCh, tree)

		// Small delay before next iteration
		time.Sleep(5 * time.Second)
	}
}

// sendTreeUpdate sends a tree update to the TUI via the program reference.
func sendTreeUpdate(_ chan<- string, nodes []*TreeNode) {
	if programRef != nil {
		programRef.Send(TreeUpdateMsg{Nodes: nodes})
	}
}

// sendStatusUpdate sends a status bar update to the TUI.
func sendStatusUpdate(streamer, status string) {
	if programRef != nil {
		programRef.Send(StatusUpdateMsg{Streamer: streamer, Status: status})
	}
}

// sendRemainingTime calculates and sends the total remaining watch time.
func sendRemainingTime(mgr *drops.Manager) {
	if programRef != nil {
		programRef.Send(RemainingTimeMsg{Seconds: mgr.TotalRemainingSeconds()})
	}
}

// syncWithCookies fetches drop progress from the server and syncs the manager,
// using already-loaded cookies (no disk I/O, no duplicate cookie parsing).
func syncWithCookies(client *kick.Client, mgr *drops.Manager, cookies map[string]string) error {
	progress, err := client.GetDropsProgress(cookies)
	if err != nil {
		return fmt.Errorf("get drops progress: %w", err)
	}
	return mgr.SyncFromServer(progress, client, cookies)
}

// buildTreeFromManager builds the tree structure from the manager's live state.
func buildTreeFromManager(
	_ *kick.CampaignsResponse,
	mgr *drops.Manager,
	_ map[int]string,
) []*TreeNode {
	rewards := mgr.GetRewards()
	if len(rewards) == 0 {
		return nil
	}

	// Build category name map from the rewards themselves
	catNames := make(map[int]string)
	for _, r := range rewards {
		if r.CategoryName != "" {
			catNames[r.CategoryID] = r.CategoryName
		}
	}

	var allRewards []RewardInfo
	for _, r := range rewards {
		catName := catNames[r.CategoryID]
		if catName == "" {
			catName = fmt.Sprintf("Category %d", r.CategoryID)
		}

		ri := RewardInfo{
			ID:             r.ID,
			CampaignID:     r.CampaignID,
			CampaignName:   r.CampaignName,
			Name:           r.Name,
			RequiredUnits:  r.RequiredUnits,
			RemainingUnits: r.RemainingUnits,
			Claimed:        r.Claimed,
			CategoryID:     r.CategoryID,
			CategoryName:   catName,
		}
		allRewards = append(allRewards, ri)
	}

	return BuildTreeFromRewards(allRewards, catNames)
}

// SetLogAdapter initializes the log adapter to send output to the given channel.
func SetLogAdapter(logCh chan string) {
	log.SetLogger(func(format string, args ...any) {
		line := fmt.Sprintf(format, args...)
		// Strip trailing newline if present (log.Println adds one)
		line = strings.TrimSuffix(line, "\n")
		select {
		case logCh <- line:
		default:
			// Drop log line if channel buffer is full
		}
	})
}
