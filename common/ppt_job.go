package common

import "time"

type PptJobState struct {
    JobID              string          `json:"job_id"`
    RequestID          string          `json:"request_id"`
    UserID             string          `json:"user_id"`
    OpenNotebookJobID  string          `json:"opennotebook_job_id"`
    Status             string          `json:"status"`
    ArtifactURL        string          `json:"artifact_url"`
    Metadata           PptJobMetadata  `json:"metadata"`
    FailedReason       string          `json:"failed_reason,omitempty"`
    QuotaPrecheckID    string          `json:"quota_precheck_id"`
    CreatedAt          time.Time       `json:"created_at"`
    UpdatedAt          time.Time       `json:"updated_at"`
    TTL                time.Time       `json:"ttl"`
}

type PptJobMetadata struct {
    Pages      int    `json:"pages,omitempty"`
    Template   string `json:"template,omitempty"`
    FileSizeBytes int64 `json:"file_size_bytes,omitempty"`
}

func PptJobRedisKey(jobID string) string {
    return "ppt_job:" + jobID
}
