package spotify

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

var (
	ErrTokenExpired = errors.New("token expired")
	ErrNotPlaying   = errors.New("not playing")
)

// TokenResponse represents response from Spotify token endpoint
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
}

// TrackInfo represents extracted track information for the overlay
type TrackInfo struct {
	Title     string `json:"title"`
	Artist    string `json:"artist"`
	AlbumArt  string `json:"album_art"`
	IsPlaying bool   `json:"is_playing"`
}

// Raw Spotify API Structures
type spotifyItem struct {
	Name    string `json:"name"`
	Artists []struct {
		Name string `json:"name"`
	} `json:"artists"`
	Album struct {
		Images []struct {
			URL    string `json:"url"`
			Height int    `json:"height"`
			Width  int    `json:"width"`
		} `json:"images"`
	} `json:"album"`
	DurationMs int `json:"duration_ms"`
}

type spotifyCurrentlyPlayingResponse struct {
	IsPlaying  bool        `json:"is_playing"`
	Item       spotifyItem `json:"item"`
	ProgressMs int         `json:"progress_ms"`
	Error      *struct {
		Status  int    `json:"status"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Client manages Spotify API interaction
type Client struct {
	httpClient *http.Client
	mu         sync.RWMutex

	clientID     string
	clientSecret string
	accessToken  string
	refreshToken string
	tokenExpiry  time.Time
}

func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// UpdateCredentials updates Spotify client ID and secret
func (c *Client) UpdateCredentials(clientID, clientSecret string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.clientID = clientID
	c.clientSecret = clientSecret
}

// SetTokens sets tokens in memory
func (c *Client) SetTokens(accessToken, refreshToken string, expiresIn int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.accessToken = accessToken
	if refreshToken != "" {
		c.refreshToken = refreshToken
	}
	if expiresIn > 0 {
		c.tokenExpiry = time.Now().Add(time.Duration(expiresIn) * time.Second)
	}
}

// GetTokens returns current tokens
func (c *Client) GetTokens() (string, string) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.accessToken, c.refreshToken
}

// ClearTokens resets current tokens (logout)
func (c *Client) ClearTokens() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.accessToken = ""
	c.refreshToken = ""
	c.tokenExpiry = time.Time{}
}

// IsAuthorized returns whether client has an access token
func (c *Client) IsAuthorized() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.accessToken != "" || c.refreshToken != ""
}

// GenerateRandomState generates random string for OAuth state parameter
func GenerateRandomState(n int) string {
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		return "state1234567890"
	}
	return hex.EncodeToString(bytes)[:n]
}

// BuildAuthorizeURL generates Spotify OAuth authorization URL
func (c *Client) BuildAuthorizeURL(redirectURI, state string) string {
	c.mu.RLock()
	clientID := c.clientID
	c.mu.RUnlock()

	params := url.Values{}
	params.Set("response_type", "code")
	params.Set("client_id", clientID)
	params.Set("scope", "user-read-private user-read-email user-read-currently-playing")
	params.Set("redirect_uri", redirectURI)
	params.Set("state", state)

	return "https://accounts.spotify.com/authorize?" + params.Encode()
}

// ExchangeCode exchanges auth code for access & refresh tokens
func (c *Client) ExchangeCode(code, redirectURI string) (*TokenResponse, error) {
	c.mu.RLock()
	clientID := c.clientID
	clientSecret := c.clientSecret
	c.mu.RUnlock()

	if clientID == "" || clientSecret == "" {
		return nil, errors.New("client ID or secret not configured")
	}

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)

	req, err := http.NewRequest("POST", "https://accounts.spotify.com/api/token", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}

	auth := base64.StdEncoding.EncodeToString([]byte(clientID + ":" + clientSecret))
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token exchange failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, err
	}

	c.SetTokens(tokenResp.AccessToken, tokenResp.RefreshToken, tokenResp.ExpiresIn)
	return &tokenResp, nil
}

// RefreshAccessToken refreshes the access token using refresh_token
func (c *Client) RefreshAccessToken(overrideRefreshToken string) (*TokenResponse, error) {
	c.mu.RLock()
	clientID := c.clientID
	clientSecret := c.clientSecret
	refreshToken := c.refreshToken
	c.mu.RUnlock()

	if overrideRefreshToken != "" {
		refreshToken = overrideRefreshToken
	}

	if clientID == "" || clientSecret == "" {
		return nil, errors.New("client ID or secret not configured")
	}
	if refreshToken == "" {
		return nil, errors.New("no refresh token available")
	}

	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)

	req, err := http.NewRequest("POST", "https://accounts.spotify.com/api/token", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}

	auth := base64.StdEncoding.EncodeToString([]byte(clientID + ":" + clientSecret))
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("refresh token failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, err
	}

	// Spotify may or may not return a new refresh token on refresh
	if tokenResp.RefreshToken == "" {
		tokenResp.RefreshToken = refreshToken
	}

	c.SetTokens(tokenResp.AccessToken, tokenResp.RefreshToken, tokenResp.ExpiresIn)
	return &tokenResp, nil
}

// GetCurrentlyPlaying retrieves current playing song information
func (c *Client) GetCurrentlyPlaying(token string) (*TrackInfo, error) {
	if token == "" {
		c.mu.RLock()
		token = c.accessToken
		c.mu.RUnlock()
	}

	if token == "" {
		return nil, errors.New("not authenticated")
	}

	req, err := http.NewRequest("GET", "https://api.spotify.com/v1/me/player/currently-playing", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, ErrTokenExpired
	}

	if resp.StatusCode == http.StatusNoContent {
		return nil, ErrNotPlaying
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("spotify currently-playing status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result spotifyCurrentlyPlayingResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	if result.Item.Name == "" {
		return nil, ErrNotPlaying
	}

	var artists []string
	for _, a := range result.Item.Artists {
		if a.Name != "" {
			artists = append(artists, a.Name)
		}
	}

	artistStr := strings.Join(artists, ", ")

	albumArt := ""
	if len(result.Item.Album.Images) > 0 {
		albumArt = result.Item.Album.Images[0].URL
	}

	return &TrackInfo{
		Title:     result.Item.Name,
		Artist:    artistStr,
		AlbumArt:  albumArt,
		IsPlaying: result.IsPlaying,
	}, nil
}
