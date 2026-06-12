package kick

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/gorilla/websocket"

	"kickautodrops/internal/log"
)

// ProgressUpdate is sent on the progress channel every 60 seconds.
type ProgressUpdate struct {
	Username      string
	WatchedMillis int // milliseconds watched since last update (normally 60,000)
}

// Connect establishes a WebSocket connection to the Kick viewer endpoint and
// maintains it for the lifetime of the context. It sends periodic ping/handshake
// messages, reports progress on progressChan, and watches for stream/game changes.
//
// Returns earlyEnd=true if the stream went offline or changed category (caller
// should find a new stream). Returns earlyEnd=false on normal context expiration.
func (c *Client) Connect(
	ctx context.Context,
	channelID int,
	username string,
	categoryID int,
	token string,
	progressChan chan<- ProgressUpdate,
) (earlyEnd bool, err error) {

	const maxRetries = 10
	baseDelay := 5 * time.Second

	for retry := 0; retry < maxRetries; retry++ {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		default:
		}

		log.Println("🔌 Connecting to WebSocket...")
		log.Printf("  → URL: wss://websockets.kick.com/viewer/v1/connect?token=%s...\n", token[:20])

		wsURL := fmt.Sprintf("wss://websockets.kick.com/viewer/v1/connect?token=%s", token)
		ws, _, err := c.wsDial.DialContext(ctx, wsURL, nil)
		if err != nil {
			log.Printf("❌ WebSocket dial error (attempt %d/%d): %v\n", retry+1, maxRetries, err)
			if isAuthError(err) {
				log.Println("  → Authentication failed - cookies may be expired")
				log.Println("  → Export fresh cookies from kick.com using the browser extension")
				return false, fmt.Errorf("websocket auth error: %w", err)
			}

			if retry < maxRetries-1 {
				wait := baseDelay + time.Duration(rand.Int63n(5001*int64(retry+1)))
				log.Printf("  → Reconnecting in %v (attempt %d/%d)\n", wait, retry+2, maxRetries)
				time.Sleep(wait)
			}
			continue
		}

		log.Println("  ✓ Connection established!")

		// Run the message loop; if it returns an error we'll retry
		log.Println("  → Starting message loop (ping/handshake every ~12s)...")
		early, loopErr := c.messageLoop(ctx, ws, channelID, username, categoryID, progressChan)
		ws.Close()

		if loopErr != nil {
			log.Printf("  ⚠ WebSocket error: %v\n", loopErr)
			if retry < maxRetries-1 {
				wait := baseDelay + time.Duration(rand.Int63n(5001))
				log.Printf("  → Reconnecting in %v (attempt %d/%d)\n", wait, retry+2, maxRetries)
				time.Sleep(wait)
			}
			continue
		}

		return early, nil
	}

	log.Println("⛔ The number of attempts has been exceeded")
	return false, fmt.Errorf("max websocket retries exceeded")
}

// messageLoop runs the ping/handshake/progress loop on an established WebSocket.
func (c *Client) messageLoop(
	ctx context.Context,
	ws *websocket.Conn,
	channelID int,
	username string,
	categoryID int,
	progressChan chan<- ProgressUpdate,
) (earlyEnd bool, err error) {

	counter := 0
	lastReport := time.Now()

	log.Printf("  [WS] Message loop started for %s (channel %d, category %d)\n", username, channelID, categoryID)

	// Get stream info to find the livestream_id
	streamInfo, err := c.GetStreamInfo(username)
	if err != nil {
		log.Printf("⚠ Error getting initial stream info: %v\n", err)
	}

	for {
		select {
		case <-ctx.Done():
			return false, nil
		default:
		}

		counter++

		// Alternate between ping and channel_handshake
		if counter%2 == 0 {
			msg := map[string]string{"type": "ping"}
			if err := ws.WriteJSON(msg); err != nil {
				return false, fmt.Errorf("write ping: %w", err)
			}
			log.Println("📤 ping")
		} else {
			msg := map[string]interface{}{
				"type": "channel_handshake",
				"data": map[string]interface{}{
					"message": map[string]int{
						"channelId": channelID,
					},
				},
			}
			if err := ws.WriteJSON(msg); err != nil {
				return false, fmt.Errorf("write handshake: %w", err)
			}
			log.Printf("🤝 handshake (channel %d)\n", channelID)
		}

		// Try to read a message (non-blocking, short timeout)
		ws.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, msgBytes, readErr := ws.ReadMessage()
		if readErr == nil && len(msgBytes) > 0 {
			preview := string(msgBytes)
			if len(preview) > 100 {
				preview = preview[:100]
			}
			log.Printf("📥 Received: %s\n", preview)
		}

		delay := 11 + rand.Intn(6) // 11-16 seconds
		log.Printf("⏳ Delay %ds\n", delay)

		// Progress reporting every 60 seconds
		now := time.Now()
		if now.Sub(lastReport) >= 60*time.Second {
			if streamInfo != nil && streamInfo.ID != nil && streamInfo.IsLive {
				event := map[string]interface{}{
					"type": "user_event",
					"data": map[string]interface{}{
						"message": map[string]interface{}{
							"name":          "tracking.user.watch.livestream",
							"channel_id":    channelID,
							"livestream_id": streamInfo.ID,
						},
					},
				}
				if err := ws.WriteJSON(event); err != nil {
					log.Printf("  ⚠ Error sending user_event: %v\n", err)
				}
			}

			// Send progress update
			select {
			case progressChan <- ProgressUpdate{Username: username, WatchedMillis: 60000}:
			default:
			}

			lastReport = now
		}

		// Random category/online check every 2-3 iterations
		if rand.Intn(3) == 0 {
			info, checkErr := c.GetStreamInfo(username)
			if checkErr != nil {
				log.Printf("  ⚠️ Category check error: %v\n", checkErr)
			} else {
				streamInfo = info // update cached info
				if !info.IsLive {
					log.Printf("  ✗ %s is offline\n", username)
					// Report remaining progress
					select {
					case progressChan <- ProgressUpdate{Username: username, WatchedMillis: 60000}:
					default:
					}
					return true, nil // early end
				}
				if info.GameID() != 0 && info.GameID() != categoryID {
					log.Printf("  ✗ %s is playing another game (category %d)\n", username, info.GameID())
					select {
					case progressChan <- ProgressUpdate{Username: username, WatchedMillis: 60000}:
					default:
					}
					return true, nil // early end
				}
				log.Printf("  ✓ %s online\n", username)
			}
		}

		// Wait for the delay or context cancellation
		timer := time.NewTimer(time.Duration(delay) * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return false, nil
		case <-timer.C:
		}
	}
}

// isAuthError checks if the error indicates an authentication problem.
func isAuthError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return contains(errStr, "403") || contains(errStr, "refused") || contains(errStr, "401")
}

// contains reports whether substr is within s (avoids importing strings just for this).
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
