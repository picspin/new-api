package opennotebook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

type SlideGenerationRequest struct {
	SlideProfile  string   `json:"slide_profile"`
	DeckName      string   `json:"deck_name"`
	Content       string   `json:"content,omitempty"`
	NotebookID    string   `json:"notebook_id,omitempty"`
	OutputFormats []string `json:"output_formats,omitempty"`
}

type SlideJobResponse struct {
	JobID   string `json:"job_id"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type SlideStatusResponse struct {
	JobID       string `json:"job_id"`
	Status      string `json:"status"`
	ErrorMessage string `json:"error_message,omitempty"`
	PptxFile    string `json:"pptx_file,omitempty"`
	PptxURL     string `json:"pptx_url,omitempty"`
}

type PodcastGenerationRequest struct {
	EpisodeProfile string `json:"episode_profile"`
	SpeakerProfile string `json:"speaker_profile"`
	EpisodeName    string `json:"episode_name"`
	Content        string `json:"content,omitempty"`
	NotebookID     string `json:"notebook_id,omitempty"`
}

type PodcastJobResponse struct {
	JobID   string `json:"job_id"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type PodcastStatusResponse struct {
	JobID       string `json:"job_id"`
	Status      string `json:"status"`
	ErrorMessage string `json:"error_message,omitempty"`
	AudioFile   string `json:"audio_file,omitempty"`
	AudioURL    string `json:"audio_url,omitempty"`
}

func (c *Client) SubmitSlideJob(ctx context.Context, req SlideGenerationRequest) (*SlideJobResponse, error) {
	// POST /slides/generate
	body, _ := json.Marshal(req)
	httpReq, _ := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/slides/generate", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("opennotebook slides submit failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("opennotebook slides submit failed status %d: %s", resp.StatusCode, string(b))
	}
	var result SlideJobResponse
	json.NewDecoder(resp.Body).Decode(&result)
	return &result, nil
}

func (c *Client) GetSlideJobStatus(ctx context.Context, jobID string) (*SlideStatusResponse, error) {
	// GET /slides/jobs/{job_id}
	httpReq, _ := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/slides/jobs/"+jobID, nil)
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("opennotebook slides status failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("opennotebook slides status failed status %d: %s", resp.StatusCode, string(b))
	}
	var result SlideStatusResponse
	json.NewDecoder(resp.Body).Decode(&result)
	return &result, nil
}

func (c *Client) SubmitPodcastJob(ctx context.Context, req PodcastGenerationRequest) (*PodcastJobResponse, error) {
	// POST /podcasts/generate
	body, _ := json.Marshal(req)
	httpReq, _ := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/podcasts/generate", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("opennotebook podcasts submit failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("opennotebook podcasts submit failed status %d: %s", resp.StatusCode, string(b))
	}
	var result PodcastJobResponse
	json.NewDecoder(resp.Body).Decode(&result)
	return &result, nil
}

func (c *Client) GetPodcastJobStatus(ctx context.Context, jobID string) (*PodcastStatusResponse, error) {
	// GET /podcasts/jobs/{job_id}
	httpReq, _ := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/podcasts/jobs/"+jobID, nil)
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("opennotebook podcasts status failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("opennotebook podcasts status failed status %d: %s", resp.StatusCode, string(b))
	}
	var result PodcastStatusResponse
	json.NewDecoder(resp.Body).Decode(&result)
	return &result, nil
}
