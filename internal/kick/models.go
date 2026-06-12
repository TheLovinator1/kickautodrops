package kick

// ---- Campaigns ----

type CampaignsResponse struct {
	Data []Campaign `json:"data"`
}

type Campaign struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Status   string    `json:"status"`
	Category Category  `json:"category"`
	Channels []Channel `json:"channels"`
	Rewards  []Reward  `json:"rewards"`
}

type Category struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type Channel struct {
	Slug string `json:"slug"`
}

type Reward struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	RequiredUnits float64 `json:"required_units"`
	Progress      float64 `json:"progress"`
	Claimed       bool    `json:"claimed"`
	ExternalID    string  `json:"external_id"`
}

// ---- Livestreams ----

type LivestreamsResponse struct {
	Data LivestreamsData `json:"data"`
}

type LivestreamsData struct {
	Livestreams []Livestream `json:"livestreams"`
}

type Livestream struct {
	Channel LivestreamChannel `json:"channel"`
}

type LivestreamChannel struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
}

// ---- Stream info (v2/channels/{username}/videos) ----

type StreamInfo struct {
	ID         interface{} `json:"id"`
	IsLive     bool        `json:"is_live"`
	Categories []Category  `json:"categories"`
}

// GameID returns the first category ID, or 0.
func (s *StreamInfo) GameID() int {
	if len(s.Categories) > 0 {
		return s.Categories[0].ID
	}
	return 0
}

// ---- Channel lookup ----

type ChannelResponse struct {
	ID int `json:"id"`
}

// ---- Token ----

type TokenResponse struct {
	Data TokenData `json:"data"`
}

type TokenData struct {
	Token string `json:"token"`
}

// ---- Drops progress ----

type DropsProgressResponse struct {
	Data []CampaignProgress `json:"data"`
}

type CampaignProgress struct {
	ID       string           `json:"id"`
	Name     string           `json:"name"`
	Status   string           `json:"status"`
	Category Category         `json:"category"`
	Rewards  []RewardProgress `json:"rewards"`
}

type RewardProgress struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Progress float64 `json:"progress"`
	Claimed  bool    `json:"claimed"`
}

// ---- Claim ----

type ClaimResponse struct {
	Message string     `json:"message"`
	Data    *ClaimData `json:"data,omitempty"`
}

type ClaimData struct {
	ID string `json:"id"`
}
