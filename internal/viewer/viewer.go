package viewer

import (
	"context"
	"fmt"
	"time"

	"kickautodrops/internal/cookies"
	"kickautodrops/internal/drops"
	"kickautodrops/internal/kick"
	"kickautodrops/internal/log"
)

// ViewStream orchestrates a full stream-watching session under the given context.
// Loads cookies → gets token → gets channel ID → starts WebSocket.
// Progress updates are recorded in the Manager.
// If cookieMap is nil, cookies are loaded from cookies.txt on disk.
func ViewStream(ctx context.Context, client *kick.Client, mgr *drops.Manager, username string, categoryID int, cookieMap map[string]string) (earlyEnd bool, err error) {
	if cookieMap == nil {
		log.Printf("  \u2192 Step 1/4: Loading cookies from disk...\n")
		var loadErr error
		cookieMap, loadErr = cookies.Load("cookies.txt")
		if loadErr != nil {
			return false, fmt.Errorf("load cookies: %w", loadErr)
		}
	} else {
		log.Printf("  \u2192 Step 1/4: Using cached cookies (%d)\n", len(cookieMap))
	}
	log.Printf("  \u2192 \u2713 Loaded %d cookies\n", len(cookieMap))

	log.Printf("  \u2192 Step 2/4: Requesting WebSocket token...\n")
	token, err := client.GetToken(cookieMap)
	if err != nil {
		return false, fmt.Errorf("get token: %w", err)
	}
	log.Printf("  \u2192 \u2713 Token obtained\n")

	log.Printf("  \u2192 Step 3/4: Looking up channel ID for %s...\n", username)
	channelID, err := client.GetChannelID(username)
	if err != nil {
		return false, fmt.Errorf("get channel ID: %w", err)
	}
	log.Printf("  \u2192 \u2713 Channel ID: %d\n", channelID)

	progressChan := make(chan kick.ProgressUpdate, 10)

	go func() {
		for update := range progressChan {
			mgr.RecordProgress(update.Username, update.WatchedMillis/1000)
		}
	}()

	log.Printf("  \u2192 Step 4/4: Connecting WebSocket for %s (category %d)...\n", username, categoryID)
	earlyEnd, err = client.Connect(ctx, channelID, username, categoryID, token, progressChan)
	close(progressChan)

	if err != nil {
		log.Printf("  \u2192 \u274c WebSocket error: %v\n", err)
	} else if earlyEnd {
		log.Printf("  \u2192 \u26a0 Stream ended early (offline/game change)\n")
	} else if ctx.Err() != nil {
		log.Printf("  \u2192 \u23f9 Cancelled\n")
	} else {
		log.Printf("  \u2192 \u2713 Session complete (timer expired)\n")
	}

	return earlyEnd, err
}

// RunWithTimer creates a context with timeout, calls ViewStream, and returns
// when either the view ends or the timer fires.
// If cookieMap is nil, ViewStream loads cookies from disk.
func RunWithTimer(
	parentCtx context.Context,
	client *kick.Client,
	mgr *drops.Manager,
	username string,
	categoryID int,
	timeout time.Duration,
	cookieMap map[string]string,
) (earlyEnd bool, err error) {

	log.Printf("  \u2192 Starting view session with %v timeout\n", timeout)
	ctx, cancel := context.WithTimeout(parentCtx, timeout)
	defer cancel()

	resultCh := make(chan struct {
		early bool
		err   error
	}, 1)

	go func() {
		early, err := ViewStream(ctx, client, mgr, username, categoryID, cookieMap)
		resultCh <- struct {
			early bool
			err   error
		}{early, err}
	}()

	select {
	case res := <-resultCh:
		if res.err != nil {
			log.Printf("  \u2192 \u274c View error: %v\n", res.err)
		} else if res.early {
			log.Printf("  \u2192 \u26a0 Stream ended early\n")
		} else if parentCtx.Err() != nil {
			log.Printf("  \u2192 \u23f9 Cancelled by signal\n")
		} else {
			log.Printf("  \u2192 \u2713 Timer expired naturally\n")
		}
		return res.early, res.err

	case <-ctx.Done():
		res := <-resultCh
		if parentCtx.Err() != nil {
			log.Println("  \u2192 \u23f9 Cancelled by signal")
		} else {
			log.Printf("  \u2192 \u23f0 Timer (%d min) expired\n", int(timeout.Minutes()))
		}
		return false, res.err
	}
}

// CheckCampaignsClaimStatus fetches drops progress and syncs the Manager's
// state, auto-claiming any fully-progressed rewards.
func CheckCampaignsClaimStatus(client *kick.Client, mgr *drops.Manager) error {
	log.Println("  → Checking drops claim status...")

	log.Println("    Loading cookies...")
	cookieMap, err := cookies.Load("cookies.txt")
	if err != nil {
		return fmt.Errorf("load cookies: %w", err)
	}
	log.Printf("    ✓ %d cookies loaded, fetching drop progress...\n", len(cookieMap))

	progress, err := client.GetDropsProgress(cookieMap)
	if err != nil {
		return fmt.Errorf("get drops progress: %w", err)
	}

	log.Println("    Syncing with local state and auto-claiming...")
	if err := mgr.SyncFromServer(progress, client, cookieMap); err != nil {
		return fmt.Errorf("sync drops: %w", err)
	}

	log.Println("  → ✓ Claim check complete")
	return nil
}
