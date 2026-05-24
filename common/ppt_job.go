package common

import (
	"encoding/json"
	"time"
)

type PptJobState struct {
	JobID             string         `json:"job_id"`
	RequestID         string         `json:"request_id"`
	UserID            string         `json:"user_id"`
	OpenNotebookJobID string         `json:"opennotebook_job_id"`
	Status            string         `json:"status"`
	ArtifactURL       string         `json:"artifact_url"`
	Metadata          PptJobMetadata `json:"metadata"`
	FailedReason      string         `json:"failed_reason,omitempty"`
	QuotaPrecheckID   string         `json:"quota_precheck_id"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
	TTL               time.Time      `json:"ttl"`
}

type PptJobMetadata struct {
	Pages          int   `json:"pages,omitempty"`
	Template       string `json:"template,omitempty"`
	FileSizeBytes int64  `json:"file_size_bytes,omitempty"`
}

func PptJobRedisKey(jobID string) string {
	return "ppt_job:" + jobID
}

// SavePptJob persists a PPT job state to Redis.
func SavePptJob(job *PptJobState) error {
	if !RedisEnabled {
		return nil
	}
	data, err := json.Marshal(job)
	if err != nil {
		return err
	}
	ttl := 24 * time.Hour
	if !job.TTL.IsZero() {
		ttl = time.Until(job.TTL)
		if ttl <= 0 {
			ttl = 24 * time.Hour
		}
	}
	return RedisSet(PptJobRedisKey(job.JobID), string(data), ttl)
}

// LoadPptJob retrieves a PPT job state from Redis.
func LoadPptJob(jobID string) (*PptJobState, error) {
	if !RedisEnabled {
		return nil, nil
	}
	data, err := RedisGet(PptJobRedisKey(jobID))
	if err != nil {
		return nil, err
	}
	var job PptJobState
	if err := json.Unmarshal([]byte(data), &job); err != nil {
		return nil, err
	}
	return &job, nil
}
