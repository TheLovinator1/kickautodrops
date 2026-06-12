package drops

import (
	"kickautodrops/internal/kick"
	"kickautodrops/internal/log"
)

// TrackedReward holds the state of a single reward during a session.
//   - Type 1: streamer-specific (has Usernames)
//   - Type 2: general category (no Usernames)
type TrackedReward struct {
	ID             string
	CampaignID     string
	CampaignName   string
	Name           string
	RequiredUnits  float64 // total minutes needed (from campaigns API)
	RemainingUnits float64 // minutes left to watch (counted down locally)
	Claimed        bool
	Type           int // 1 = streamer, 2 = general
	Usernames      []string
	CategoryID     int
	CategoryName   string
}

// StreamerTarget returned by CollectUsernames for the farming loop.
type StreamerTarget struct {
	Username        string
	RequiredSeconds int
	Claimed         bool
}

// Manager holds in-memory state of all tracked rewards.
type Manager struct {
	rewards []TrackedReward
}

// NewManager creates a Manager from the campaigns API response.
func NewManager(campaigns *kick.CampaignsResponse) *Manager {
	m := &Manager{}

	for _, campaign := range campaigns.Data {
		if campaign.Status == "expired" {
			log.Printf("  \u23ed Skipping expired campaign: %s (ID: %s)\n", campaign.Name, campaign.ID)
			continue
		}

		catID := campaign.Category.ID
		if catID == 0 {
			continue
		}

		channels := campaign.Channels
		hasChannels := len(channels) > 0

		usernames := make([]string, 0, len(channels))
		for _, ch := range channels {
			if ch.Slug != "" {
				usernames = append(usernames, ch.Slug)
			}
		}

		for _, reward := range campaign.Rewards {
			r := TrackedReward{
				ID:             reward.ID,
				CampaignID:     campaign.ID,
				CampaignName:   campaign.Name,
				Name:           reward.Name,
				RequiredUnits:  reward.RequiredUnits,
				RemainingUnits: reward.RequiredUnits,
				Type:           2,
				CategoryID:     catID,
				CategoryName:   campaign.Category.Name,
			}

			if hasChannels {
				r.Type = 1
				r.Usernames = usernames
			}

			m.rewards = append(m.rewards, r)
		}
	}

	return m
}
