package kick

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"

	"kickautodrops/internal/log"
)

// ---- Campaigns ----

func (c *Client) GetAllCampaigns() (*CampaignsResponse, error) {
	log.Println("  [API] GET https://web.kick.com/api/v1/drops/campaigns")
	req, err := c.NewRequest(http.MethodGet, "https://web.kick.com/api/v1/drops/campaigns", nil)
	if err != nil {
		return nil, fmt.Errorf("build campaigns request: %w", err)
	}

	resp, err := c.Do(req, 3)
	if err != nil {
		return nil, fmt.Errorf("get campaigns: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read campaigns body: %w", err)
	}

	var result CampaignsResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode campaigns: %w", err)
	}

	log.Printf("  [API] ✓ Got %d campaigns\n", len(result.Data))
	return &result, nil
}

// ---- Livestreams ----

func (c *Client) GetRandomStreamFromCategory(categoryID, limit int) (*Livestream, error) {
	url := fmt.Sprintf("https://web.kick.com/api/v1/livestreams?limit=%d&sort=viewer_count_desc&category_id=%d", limit, categoryID)
	log.Printf("  [API] GET %s\n", url)
	req, err := c.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build livestreams request: %w", err)
	}

	resp, err := c.Do(req, 3)
	if err != nil {
		return nil, fmt.Errorf("get livestreams: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read livestreams body: %w", err)
	}

	var result LivestreamsResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode livestreams: %w", err)
	}

	streams := result.Data.Livestreams
	if len(streams) == 0 {
		return nil, fmt.Errorf("no livestreams found for category %d", categoryID)
	}
	log.Printf("  [API] ✓ Got %d livestreams in category %d\n", len(streams), categoryID)

	// Pick a random stream from the first 1-4 (or fewer if list is small)
	maxIdx := len(streams) - 1
	idx := 1
	if maxIdx < 1 {
		idx = 0
	} else if maxIdx < 4 {
		idx = rand.Intn(maxIdx + 1)
	} else {
		idx = 1 + rand.Intn(4) // 1..4
	}

	chosen := streams[idx]
	log.Printf("  [API] → Picked streamer #%d: %s (channel %d)\n", idx, chosen.Channel.Username, chosen.Channel.ID)
	return &chosen, nil
}

// ---- Stream info ----

func (c *Client) GetStreamInfo(username string) (*StreamInfo, error) {
	url := fmt.Sprintf("https://kick.com/api/v2/channels/%s/videos", username)
	log.Printf("  [API] GET %s\n", url)
	req, err := c.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build stream info request: %w", err)
	}

	resp, err := c.Do(req, 3)
	if err != nil {
		return nil, fmt.Errorf("get stream info: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read stream info body: %w", err)
	}

	var videos []StreamInfo
	if err := json.Unmarshal(body, &videos); err != nil {
		return nil, fmt.Errorf("decode stream info: %w", err)
	}

	if len(videos) == 0 {
		log.Printf("  [API] ✓ %s returned 0 videos (offline?)\n", username)
		return &StreamInfo{}, nil
	}

	info := &videos[0]
	status := "offline"
	if info.IsLive {
		status = fmt.Sprintf("LIVE (game %d)", info.GameID())
	}
	log.Printf("  [API] ✓ %s is %s\n", username, status)
	return info, nil
}

// ---- Channel ID ----

func (c *Client) GetChannelID(channelName string) (int, error) {
	url := fmt.Sprintf("https://kick.com/api/v2/channels/%s", channelName)
	log.Printf("  [API] GET %s\n", url)
	req, err := c.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return 0, fmt.Errorf("build channel request: %w", err)
	}

	resp, err := c.Do(req, 3)
	if err != nil {
		return 0, fmt.Errorf("get channel: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("read channel body: %w", err)
	}

	var channel ChannelResponse
	if err := json.Unmarshal(body, &channel); err != nil {
		return 0, fmt.Errorf("decode channel: %w", err)
	}

	if channel.ID == 0 {
		return 0, fmt.Errorf("channel %s not found", channelName)
	}

	log.Printf("  [API] ✓ Channel ID for %s: %d\n", channelName, channel.ID)
	return channel.ID, nil
}

// ---- WebSocket token ----

func (c *Client) GetToken(cookies map[string]string) (string, error) {
	if _, ok := cookies["session_token"]; !ok {
		return "", fmt.Errorf("session_token not found in cookies")
	}

	log.Printf("  [API] Session token found: %s...\n", cookies["session_token"][:30])
	log.Println("  [API] GET https://websockets.kick.com/viewer/v1/token")

	req, err := c.NewRequest(http.MethodGet, "https://websockets.kick.com/viewer/v1/token", cookies)
	if err != nil {
		return "", fmt.Errorf("build token request: %w", err)
	}
	SetAuthHeader(req, cookies)

	req.Header.Set("X-Client-Token", "e1393935a959b4020a4491574f6490129f678acdaa92760471263db43487f823")
	req.Header.Set("Sec-Fetch-Site", "same-site")

	resp, err := c.Do(req, 5)
	if err != nil {
		return "", fmt.Errorf("get token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read token body: %w", err)
	}

	var result TokenResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("decode token: %w", err)
	}

	if result.Data.Token == "" {
		return "", fmt.Errorf("token not found in response")
	}

	log.Println("  [API] ✓ WebSocket token received")
	return result.Data.Token, nil
}

// ---- Drops progress ----

func (c *Client) GetDropsProgress(cookies map[string]string) (*DropsProgressResponse, error) {
	if _, ok := cookies["session_token"]; !ok {
		return nil, fmt.Errorf("session_token not found in cookies")
	}

	log.Println("  [API] GET https://web.kick.com/api/v1/drops/progress")
	req, err := c.NewRequest(http.MethodGet, "https://web.kick.com/api/v1/drops/progress", cookies)
	if err != nil {
		return nil, fmt.Errorf("build progress request: %w", err)
	}
	SetAuthHeader(req, cookies)
	req.Header.Set("X-Client-Token", "e1393935a959b4020a4491574f6490129f678acdaa92760471263db43487f823")
	req.Header.Set("Sec-Fetch-Site", "same-site")

	resp, err := c.Do(req, 3)
	if err != nil {
		return nil, fmt.Errorf("get drops progress: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read progress body: %w", err)
	}

	var result DropsProgressResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode progress: %w", err)
	}

	log.Printf("  [API] ✓ Got progress for %d campaigns\n", len(result.Data))
	return &result, nil
}

// ---- Claim reward ----

func (c *Client) ClaimDropReward(rewardID, campaignID string, cookies map[string]string) (*ClaimResponse, error) {
	if _, ok := cookies["session_token"]; !ok {
		return nil, fmt.Errorf("session_token not found in cookies")
	}

	log.Printf("  [API] POST https://web.kick.com/api/v1/drops/claim (reward=%s, campaign=%s)\n", rewardID, campaignID)
	req, err := c.NewRequest(http.MethodPost, "https://web.kick.com/api/v1/drops/claim", cookies)
	if err != nil {
		return nil, fmt.Errorf("build claim request: %w", err)
	}
	SetAuthHeader(req, cookies)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Client-Token", "e1393935a959b4020a4491574f6490129f678acdaa92760471263db43487f823")
	req.Header.Set("Sec-Fetch-Site", "same-site")

	payload := fmt.Sprintf(`{"reward_id":"%s","campaign_id":"%s"}`, rewardID, campaignID)
	req.Body = io.NopCloser(strings.NewReader(payload))

	resp, err := c.Do(req, 3)
	if err != nil {
		return nil, fmt.Errorf("claim reward: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read claim body: %w", err)
	}

	var result ClaimResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode claim: %w", err)
	}

	if result.Message == "Success" {
		log.Printf("  [API] ✓ Reward %s claimed successfully\n", rewardID)
	} else {
		log.Printf("  [API] ⚠ Claim response: %s\n", result.Message)
	}

	return &result, nil
}
