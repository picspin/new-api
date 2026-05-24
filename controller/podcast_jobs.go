package controller

import (
	"net/http"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/internal/opennotebook"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

// PodcastJobController handles Podcast job polling.
type PodcastJobController struct {
	adapter *service.PodcastAdapter
}

// NewPodcastJobController creates a new PodcastJobController.
func NewPodcastJobController() *PodcastJobController {
	onbClient := opennotebook.NewClient(common.GetEnvOrDefaultString("OPEN_NOTEBOOK_BASE_URL", "http://internal-notebook-api:8080"))
	return &PodcastJobController{
		adapter: service.NewPodcastAdapter(onbClient),
	}
}

// GetJobStatus retrieves the status of a Podcast generation job.
func (ctrl *PodcastJobController) GetJobStatus(ginCtx *gin.Context) {
	jobID := ginCtx.Param("job_id")
	if jobID == "" {
		ginCtx.JSON(http.StatusBadRequest, gin.H{"error": "job_id is required"})
		return
	}

	job, err := common.LoadPodcastJob(jobID)
	if err != nil {
		ginCtx.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
		return
	}

	if job.UserID != strconv.Itoa(ginCtx.GetInt("id")) {
		ginCtx.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	updatedJob, err := ctrl.adapter.GetPodcastJobStatus(ginCtx.Request.Context(), *job)
	if err != nil {
		ginCtx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update job status"})
		return
	}

	_ = common.SavePodcastJob(updatedJob)

	ginCtx.JSON(http.StatusOK, gin.H{
		"job_id":        updatedJob.JobID,
		"status":        updatedJob.Status,
		"artifact_url":  updatedJob.ArtifactURL,
		"failed_reason": updatedJob.FailedReason,
		"updated_at":    updatedJob.UpdatedAt.Format(time.RFC3339),
	})
}
