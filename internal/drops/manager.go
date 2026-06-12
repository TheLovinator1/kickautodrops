package drops

import (
	"math"

	"kickautodrops/internal/kick"
	"kickautodrops/internal/log"
)

// ---- Collect usernames ----

// CollectUsernames returns all type-1 (streamer) targets that are not yet claimed
// and still have remaining time.
func (m *Manager) CollectUsernames() []StreamerTarget {
	var targets []StreamerTarget

	for _, r := range m.rewards {
		if r.Type != 1 || len(r.Usernames) == 0 {
			continue
		}
		for _, username := range r.Usernames {
			targets = append(targets, StreamerTarget{
				Username:        username,
				RequiredSeconds: int(r.RemainingUnits * 60),
				Claimed:         r.Claimed,
			})
		}
	}

	return targets
}

// ---- Get remaining time ----

// GetRemainingTime returns remaining seconds for a username, falling back to
// the type=2 (general) reward if no streamer-specific match is found.
func (m *Manager) GetRemainingTime(username string) int {
	for _, r := range m.rewards {
		if r.Type == 1 && contains(r.Usernames, username) {
			return int(r.RemainingUnits * 60)
		}
	}
	// Fallback to general
	for _, r := range m.rewards {
		if r.Type == 2 {
			return int(r.RemainingUnits * 60)
		}
	}
	return 0
}

// ---- Record progress ----

// RecordProgress decrements the remaining units for the given username by the
// watched seconds. Falls back to the type=2 (general) reward if no match.
func (m *Manager) RecordProgress(username string, watchedSeconds int) {
	watchedMinutes := round(float64(watchedSeconds)/60.0, 1)

	for i := range m.rewards {
		r := &m.rewards[i]
		if r.Type == 1 && contains(r.Usernames, username) {
			old := r.RemainingUnits
			r.RemainingUnits = round(math.Max(0, old-watchedMinutes), 1)
			log.Printf("  [Progress] %s: %.1f → %.1f min remaining (-%.1f min watched)\n",
				username, old, r.RemainingUnits, watchedMinutes)
			return
		}
	}

	// Fallback to general
	for i := range m.rewards {
		r := &m.rewards[i]
		if r.Type == 2 {
			old := r.RemainingUnits
			r.RemainingUnits = round(math.Max(0, old-watchedMinutes), 1)
			log.Printf("  [Progress] General: %.1f → %.1f min remaining (-%.1f min watched, via fallback)\n",
				old, r.RemainingUnits, watchedMinutes)
			return
		}
	}

	log.Printf("⚠️ %s not found, no general entry either\n", username)
}

// ---- Sync from server ----

// SyncFromServer fetches the current progress from the server and auto-claims
// any fully-progressed rewards. Updates the Manager's claim state and
// RemainingUnits in place so the TUI tree shows accurate progress.
func (m *Manager) SyncFromServer(progress *kick.DropsProgressResponse, client *kick.Client, cookies map[string]string) error {
	// Index manager rewards by ID for fast lookup
	byID := make(map[string]*TrackedReward, len(m.rewards))
	for i := range m.rewards {
		byID[m.rewards[i].ID] = &m.rewards[i]
	}

	serverClaimed := make(map[string]bool)

	for _, campaign := range progress.Data {
		if campaign.Status == "expired" {
			continue
		}

		log.Printf("  Campaign: %s (ID: %s) [Status: %s]\n",
			campaign.Name, campaign.ID, campaign.Status)

		for _, reward := range campaign.Rewards {
			log.Printf("    Reward: %s (progress=%.1f, claimed=%v)\n",
				reward.Name, reward.Progress, reward.Claimed)

			// Sync RemainingUnits from server progress
			if local := byID[reward.ID]; local != nil {
				oldRemaining := local.RemainingUnits
				// Server progress is 0.0–1.0; compute remaining minutes
				local.RemainingUnits = round(local.RequiredUnits*(1.0-reward.Progress), 1)
				if local.RemainingUnits != oldRemaining {
					log.Printf("    → Synced progress: %.1f → %.1f min remaining (%.0f%%)\n",
						oldRemaining, local.RemainingUnits, reward.Progress*100)
				}
			}

			if reward.Progress >= 1 && !reward.Claimed {
				log.Printf("    → Progress is 100%% and reward is unclaimed! Attempting to claim...\n")
				claimResult, err := client.ClaimDropReward(reward.ID, campaign.ID, cookies)
				if err != nil {
					log.Printf("    ✗ Failed to claim %s: %v\n", reward.Name, err)
				} else if claimResult.Message == "Success" {
					serverClaimed[reward.ID] = true
					log.Printf("    ✓ Claimed: %s\n", reward.Name)
				} else {
					log.Printf("    ✗ Failed: %s (message: %s)\n", reward.Name, claimResult.Message)
				}
			} else if reward.Claimed && reward.Progress >= 1 {
				log.Printf("    → Already claimed on server, marking locally\n")
				serverClaimed[reward.ID] = true
			} else if reward.Progress < 1 {
				log.Printf("    → Progress %.0f%% complete, need 100%% before claiming\n", reward.Progress*100)
			}
		}
	}

	updated := 0
	for i := range m.rewards {
		r := &m.rewards[i]
		if serverClaimed[r.ID] && !r.Claimed {
			r.Claimed = true
			updated++
			log.Printf("  → Marked reward %s as claimed locally\n", r.ID)
		}
	}

	log.Printf("\n  → Sync complete: %d rewards updated\n", updated)
	return nil
}

// TotalRemainingSeconds returns the sum of remaining seconds for all
// unclaimed rewards. Returns 0 if everything is claimed or empty.
func (m *Manager) TotalRemainingSeconds() int {
	var total float64
	for _, r := range m.rewards {
		if !r.Claimed {
			total += r.RemainingUnits * 60
		}
	}
	return int(total)
}

// GetRewards returns a snapshot of all tracked rewards.
// The TUI uses this to build the drops tree with live progress/claim status.
func (m *Manager) GetRewards() []TrackedReward {
	out := make([]TrackedReward, len(m.rewards))
	copy(out, m.rewards)
	return out
}

// ---- Helpers ----

func round(val float64, decimals int) float64 {
	pow := math.Pow(10, float64(decimals))
	return math.Round(val*pow) / pow
}

func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
