package common

import (
	"encoding/json"
	"time"
)

type PodcastJobState struct {
	JobID             string             `json:"job_id"`
	RequestID         string             `json:"request_id"`
	UserID            string             `json:"user_id"`
	OpenNotebookJobID string             `json:"opennotebook_job_id"`
	Status            string             `json:"status"`
	ArtifactURL       string             `json:"artifact_url"`
	Metadata          PodcastJobMetadata `json:"metadata"`
	FailedReason      string             `json:"failed_reason,omitempty"`
	QuotaPrecheckID   string             `json:"quota_precheck_id"`
	CreatedAt         time.Time          `json:"created_at"`
	UpdatedAt         time.Time          `json:"updated_at"`
	TTL               time.Time          `json:"ttl"`
}

type PodcastJobMetadata struct {
	DurationSeconds int    `json:"duration_seconds,omitempty"`
	Voice           string `json:"voice,omitempty"`
	Language        string `json:"language,omitempty"`
}

func PodcastJobRedisKey(jobID string) string {
	return "podcast_job:" + jobID
}

func SavePodcastJob(job *PodcastJobState) error {
	data, err := json.Marshal(job)
	if err != nil {
		return err
	}
	return RedisSet(PodcastJobRedisKey(job.JobID), string(data), 7*24*time.Hour)
}

func LoadPodcastJob(jobID string) (*PodcastJobState, error) {
	data, err := RedisGet(PodcastJobRedisKey(jobID))
	if err != nil {
		return nil, err
	}
	var job PodcastJobState
	err = json.Unmarshal([]byte(data), &job)
	if err != nil {
		return nil, err
	}
	return &job, nil
}
