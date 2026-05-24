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

// PPTJobController handles PPT job polling.
type PPTJobController struct {
	adapter *service.PptAdapter
}

// NewPPTJobController creates a new PPTJobController.
func NewPPTJobController() *PPTJobController {
	onbClient := opennotebook.NewClient(common.GetEnvOrDefaultString("OPEN_NOTEBOOK_BASE_URL", "http://internal-notebook-api:8080"))
	return &PPTJobController{
		adapter: service.NewPptAdapter(onbClient),
	}
}

// GetJobStatus retrieves the status of a PPT generation job.
func (ctrl *PPTJobController) GetJobStatus(ginCtx *gin.Context) {
	jobID := ginCtx.Param("job_id")
	if jobID == "" {
		ginCtx.JSON(http.StatusBadRequest, gin.H{"error": "job_id is required"})
		return
	}

	job, err := common.LoadPptJob(jobID)
	if err != nil {
		ginCtx.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
		return
	}

	if job.UserID != strconv.Itoa(ginCtx.GetInt("id")) {
		ginCtx.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	updatedJob, err := ctrl.adapter.GetPptJobStatus(ginCtx.Request.Context(), *job)
	if err != nil {
		ginCtx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update job status"})
		return
	}

	_ = common.SavePptJob(updatedJob)

	ginCtx.JSON(http.StatusOK, gin.H{
		"job_id":        updatedJob.JobID,
		"status":        updatedJob.Status,
		"artifact_url":  updatedJob.ArtifactURL,
		"failed_reason": updatedJob.FailedReason,
		"updated_at":    updatedJob.UpdatedAt.Format(time.RFC3339),
	})
}
