package kodi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	HostURL    string
	Username   string
	Password   string
	HTTPClient *http.Client
}

func NewClient(hostURL, username, password string) *Client {
	return &Client{
		HostURL:  hostURL,
		Username: username,
		Password: password,
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

type JsonRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
	ID      int         `json:"id"`
}

type JsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result"`
	Error   *JsonRPCError   `json:"error,omitempty"`
	ID      int             `json:"id"`
}

type JsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type MediaItem struct {
	ID        int               `json:"id"`
	Label     string            `json:"label"`
	Title     string            `json:"title"`
	Rating    float64           `json:"rating,omitempty"`
	Year      int               `json:"year,omitempty"`
	Plot      string            `json:"plot,omitempty"`
	Runtime   int               `json:"runtime,omitempty"`
	Duration  int               `json:"duration,omitempty"` // Fallback for some Kodi versions
	Thumbnail string            `json:"thumbnail,omitempty"`
	Art       map[string]string `json:"art,omitempty"` // Added Art map

	StreamDetails *StreamDetails `json:"streamdetails,omitempty"` // Deeply nested duration

	ShowTitle    string `json:"showtitle,omitempty"`
	Season       int    `json:"season,omitempty"`
	Episode      int    `json:"episode,omitempty"`
	EpisodeCount int    `json:"episode_count,omitempty"`
}

type StreamDetails struct {
	Video []struct {
		Duration int `json:"duration"`
	} `json:"video"`
}

func (m *MediaItem) UnmarshalJSON(data []byte) error {
	type Alias MediaItem
	aux := &struct {
		MovieID   int `json:"movieid"`
		EpisodeID int `json:"episodeid"`
		TVShowID  int `json:"tvshowid"`
		SeasonID  int `json:"seasonid"`
		Episodes  int `json:"episode"`
		*Alias
	}{
		Alias: (*Alias)(m),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	if aux.MovieID != 0 {
		m.ID = aux.MovieID
	} else if aux.EpisodeID != 0 {
		m.ID = aux.EpisodeID
		m.Episode = aux.Episodes
	} else if aux.TVShowID != 0 {
		m.ID = aux.TVShowID
		m.EpisodeCount = aux.Episodes
	} else if aux.SeasonID != 0 {
		m.ID = aux.SeasonID
		m.EpisodeCount = aux.Episodes
	}

	// Logic to pick the best runtime info
	if m.Runtime == 0 {
		if m.Duration != 0 {
			m.Runtime = m.Duration
		} else if m.StreamDetails != nil && len(m.StreamDetails.Video) > 0 {
			m.Runtime = m.StreamDetails.Video[0].Duration
		}
	}

	if m.Title == "" && m.Label != "" {
		m.Title = m.Label
	}
	return nil
}

func (c *Client) GetMovies() ([]MediaItem, error) {
	if c.HostURL == "mock" {
		return []MediaItem{
			{ID: 1, Title: "The Matrix", Year: 1999, Rating: 8.7, Runtime: 8160, Thumbnail: "https://www.themoviedb.org/t/p/w600_and_h900_bestv2/f89U3Y9YvYvwsf9qTMRS9XBt7qy.jpg"},
			{ID: 2, Title: "Inception", Year: 2010, Rating: 8.8, Runtime: 8880, Thumbnail: "https://www.themoviedb.org/t/p/w600_and_h900_bestv2/edv5CZv0jH9upBPaY6PeBjj9d7A.jpg"},
		}, nil
	}
	params := map[string]interface{}{"properties": []string{"title", "year", "rating", "plot", "runtime", "thumbnail", "art"}}
	req := JsonRPCRequest{JSONRPC: "2.0", Method: "VideoLibrary.GetMovies", Params: params, ID: 1}
	var resp JsonRPCResponse
	if err := c.sendRequest(req, &resp); err != nil {
		return nil, err
	}
	var result struct {
		Movies []MediaItem `json:"movies"`
	}
	result.Movies = []MediaItem{} // Initialize to avoid null
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		slog.Error("Error unmarshaling movies", "error", err)
		return result.Movies, nil
	}
	return result.Movies, nil
}

func (c *Client) GetTVShows() ([]MediaItem, error) {
	if c.HostURL == "mock" {
		return []MediaItem{
			{ID: 201, Title: "Breaking Bad", Year: 2008, Rating: 9.5, EpisodeCount: 62, Thumbnail: "https://www.themoviedb.org/t/p/w600_and_h900_bestv2/ggws000vxiO0Hcm37m0B3m6idXN.jpg"},
			{ID: 202, Title: "The Office", Year: 2005, Rating: 8.9, EpisodeCount: 201, Thumbnail: "https://www.themoviedb.org/t/p/w600_and_h900_bestv2/7D980V87m274Y6968mY96Jvwpis.jpg"},
		}, nil
	}
	params := map[string]interface{}{"properties": []string{"title", "year", "rating", "plot", "thumbnail", "episode", "art"}}
	req := JsonRPCRequest{JSONRPC: "2.0", Method: "VideoLibrary.GetTVShows", Params: params, ID: 3}
	var resp JsonRPCResponse
	if err := c.sendRequest(req, &resp); err != nil {
		return nil, err
	}
	var result struct {
		TVShows []MediaItem `json:"tvshows"`
	}
	result.TVShows = []MediaItem{}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		slog.Error("Error unmarshaling tvshows", "error", err)
		return result.TVShows, nil
	}
	return result.TVShows, nil
}

func (c *Client) GetSeasons(tvshowid int) ([]MediaItem, error) {
	if c.HostURL == "mock" {
		return []MediaItem{{ID: 20101, Title: "Season 1", Season: 1, EpisodeCount: 7, ShowTitle: "Breaking Bad"}}, nil
	}
	params := map[string]interface{}{"tvshowid": tvshowid, "properties": []string{"season", "episode", "thumbnail", "showtitle"}}
	req := JsonRPCRequest{JSONRPC: "2.0", Method: "VideoLibrary.GetSeasons", Params: params, ID: 4}
	var resp JsonRPCResponse
	if err := c.sendRequest(req, &resp); err != nil {
		return nil, err
	}
	var result struct {
		Seasons []MediaItem `json:"seasons"`
	}
	result.Seasons = []MediaItem{} // Initialize to avoid null
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		slog.Error("Error unmarshaling seasons", "error", err)
		return result.Seasons, nil
	}
	return result.Seasons, nil
}

func (c *Client) GetEpisodes(tvshowid int, season int) ([]MediaItem, error) {
	if c.HostURL == "mock" {
		return []MediaItem{{ID: 1001, Title: "Pilot", Season: 1, Episode: 1, Runtime: 3480, Rating: 9.2}}, nil
	}
	params := map[string]interface{}{"tvshowid": tvshowid, "season": season, "properties": []string{"title", "season", "episode", "runtime", "rating", "streamdetails"}}
	req := JsonRPCRequest{JSONRPC: "2.0", Method: "VideoLibrary.GetEpisodes", Params: params, ID: 5}
	var resp JsonRPCResponse
	if err := c.sendRequest(req, &resp); err != nil {
		return nil, err
	}
	var result struct {
		Episodes []MediaItem `json:"episodes"`
	}
	result.Episodes = []MediaItem{} // Initialize to avoid null
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		slog.Error("Error unmarshaling episodes", "error", err)
		return result.Episodes, nil
	}
	return result.Episodes, nil
}

func (c *Client) sendRequest(req JsonRPCRequest, resp interface{}) error {
	body, _ := json.Marshal(req)
	target := c.HostURL + "/jsonrpc"
	if !strings.HasPrefix(target, "http") {
		target = "http://" + target
	}

	httpReq, _ := http.NewRequest("POST", target, bytes.NewBuffer(body))
	httpReq.Header.Set("Content-Type", "application/json")
	if c.Username != "" {
		httpReq.SetBasicAuth(c.Username, c.Password)
	}

	httpResp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer httpResp.Body.Close()

	if err := json.NewDecoder(httpResp.Body).Decode(resp); err != nil {
		return err
	}

	// Check for RPC error
	r, ok := resp.(*JsonRPCResponse)
	if ok && r.Error != nil {
		return fmt.Errorf("kodi rpc error: %s (code: %d)", r.Error.Message, r.Error.Code)
	}

	return nil
}
