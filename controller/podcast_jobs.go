package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// PodcastJobController handles Podcast job polling.
type PodcastJobController struct{}

// NewPodcastJobController creates a new PodcastJobController.
func NewPodcastJobController() *PodcastJobController {
	return &PodcastJobController{}
}

// GetJobStatus retrieves the status of a Podcast generation job.
func (c *PodcastJobController) GetJobStatus(ginCtx *gin.Context) {
	jobID := ginCtx.Param("job_id")
	if jobID == "" {
		ginCtx.JSON(http.StatusBadRequest, gin.H{"error": "job_id is required"})
		return
	}

	// Job persistence is not wired yet.
	ginCtx.JSON(http.StatusNotImplemented, gin.H{"error": "job persistence not wired yet"})
}
