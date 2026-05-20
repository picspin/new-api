package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/new-api/common"
	"github.com/new-api/internal/opennotebook"
)

type PodcastAdapter struct {
	onbClient *opennotebook.Client
}

func NewPodcastAdapter(onbClient *opennotebook.Client) *PodcastAdapter {
	return &PodcastAdapter{onbClient: onbClient}
}

type PodcastExecuteInput struct {
	SourceRefs      []string
	Voice           string
	Language        string
	MaxDurationSecs int
}

func (a *PodcastAdapter) CreatePodcastJob(ctx context.Context, reqID, userID string, input PodcastExecuteInput) (*common.PodcastJobState, error) {
	jobID := uuid.New().String()

	req := opennotebook.PodcastGenerationRequest{
		EpisodeProfile: "default",
		SpeakerProfile: input.Voice,
		EpisodeName:    fmt.Sprintf("episode_%s", jobID),
		Content:        "",
	}

	res, err := a.onbClient.SubmitPodcastJob(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to submit podcast job: %w", err)
	}

	now := time.Now()
	return &common.PodcastJobState{
		JobID:             jobID,
		RequestID:         reqID,
		UserID:            userID,
		OpenNotebookJobID: res.JobID,
		Status:            "pending",
		ArtifactURL:       "",
		Metadata: common.PodcastJobMetadata{
			DurationSeconds: input.MaxDurationSecs,
			Voice:           input.Voice,
			Language:        input.Language,
		},
		QuotaPrecheckID: "",
		CreatedAt:       now,
		UpdatedAt:       now,
		TTL:             now.Add(7 * 24 * time.Hour),
	}, nil
}

func (a *PodcastAdapter) GetPodcastJobStatus(ctx context.Context, job common.PodcastJobState) (*common.PodcastJobState, error) {
	if job.Status != "pending" && job.Status != "processing" {
		return &job, nil
	}

	res, err := a.onbClient.GetPodcastJobStatus(ctx, job.OpenNotebookJobID)
	if err != nil {
		return nil, fmt.Errorf("failed to get podcast job status: %w", err)
	}

	updated := job
	updated.Status = MapOpenNotebookStatus(res.Status)
	updated.UpdatedAt = time.Now()

	if updated.Status == "completed" {
		updated.ArtifactURL = res.AudioURL
	} else if updated.Status == "failed" {
		updated.FailedReason = res.ErrorMessage
	}

	return &updated, nil
}

func MapOpenNotebookStatus(status string) string {
	switch status {
	case "pending", "queued":
		return "pending"
	case "processing", "running":
		return "processing"
	case "completed", "success":
		return "completed"
	case "failed", "error":
		return "failed"
	default:
		return "failed"
	}
}
