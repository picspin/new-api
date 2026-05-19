package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/new-api/common"
	"github.com/new-api/internal/opennotebook"
)

type PptAdapter struct {
	onbClient *opennotebook.Client
}

func NewPptAdapter(onbClient *opennotebook.Client) *PptAdapter {
	return &PptAdapter{onbClient: onbClient}
}

type PptExecuteInput struct {
	SourceRefs []string
	Template   string
	Style      string
	Slides     int
}

func (a *PptAdapter) CreatePptJob(ctx context.Context, reqID, userID string, input PptExecuteInput) (*common.PptJobState, error) {
	jobID := uuid.New().String()

	req := opennotebook.SlideGenerationRequest{
		SlideProfile:  input.Template, // Mapping template to profile
		DeckName:      fmt.Sprintf("deck_%s", jobID),
		OutputFormats: []string{"pptx"},
		Content:       "",
	}

	res, err := a.onbClient.SubmitSlideJob(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to submit ppt job: %w", err)
	}

	now := time.Now()
	return &common.PptJobState{
		JobID:             jobID,
		RequestID:         reqID,
		UserID:            userID,
		OpenNotebookJobID: res.JobID,
		Status:            "pending",
		Metadata: common.PptJobMetadata{
			Pages:    input.Slides,
			Template: input.Template,
		},
		CreatedAt: now,
		UpdatedAt: now,
		TTL:       now.Add(7 * 24 * time.Hour),
	}, nil
}

func (a *PptAdapter) GetPptJobStatus(ctx context.Context, job common.PptJobState) (*common.PptJobState, error) {
	if job.Status != "pending" && job.Status != "processing" {
		return &job, nil
	}

	res, err := a.onbClient.GetSlideJobStatus(ctx, job.OpenNotebookJobID)
	if err != nil {
		return nil, fmt.Errorf("failed to get ppt job status: %w", err)
	}

	updated := job
	updated.Status = MapOpenNotebookStatus(res.Status)
	updated.UpdatedAt = time.Now()

	if updated.Status == "completed" {
		updated.ArtifactURL = res.PptxURL
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
