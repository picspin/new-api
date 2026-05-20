package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// PPTJobController handles PPT job polling.
type PPTJobController struct{}

// NewPPTJobController creates a new PPTJobController.
func NewPPTJobController() *PPTJobController {
	return &PPTJobController{}
}

// GetJobStatus retrieves the status of a PPT generation job.
func (c *PPTJobController) GetJobStatus(ginCtx *gin.Context) {
	jobID := ginCtx.Param("job_id")
	if jobID == "" {
		ginCtx.JSON(http.StatusBadRequest, gin.H{"error": "job_id is required"})
		return
	}

	// Job persistence is not wired yet.
	ginCtx.JSON(http.StatusNotImplemented, gin.H{"error": "job persistence not wired yet"})
}
