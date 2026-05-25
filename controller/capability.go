package controller

import (
	"encoding/json"
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/internal/opennotebook"
	"github.com/QuantumNous/new-api/internal/surreal"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

type CapabilityExecuteController struct {
	pptAdapter       *service.PptAdapter
	podcastAdapter   *service.PodcastAdapter
	groundingAssembler *service.GroundingAssembler
	surrealOnce      sync.Once
	surrealClient    *surreal.Client
	surrealErr       error
}

func NewCapabilityExecuteController() *CapabilityExecuteController {
	return &CapabilityExecuteController{
		pptAdapter:     service.NewPptAdapter(opennotebook.NewClient(common.GetEnvOrDefaultString("OPEN_NOTEBOOK_BASE_URL", "http://internal-notebook-api:8080"))),
		podcastAdapter: service.NewPodcastAdapter(opennotebook.NewClient(common.GetEnvOrDefaultString("OPEN_NOTEBOOK_BASE_URL", "http://internal-notebook-api:8080"))),
	}
}

func (ctrl *CapabilityExecuteController) getSurrealClient() *surreal.Client {
	ctrl.surrealOnce.Do(func() {
		ctrl.surrealClient, ctrl.surrealErr = surreal.NewClient(
			common.GetEnvOrDefaultString("SURREALDB_URL", "ws://surrealdb:8000"),
			common.GetEnvOrDefaultString("SURREALDB_USER", "root"),
			common.GetEnvOrDefaultString("SURREALDB_PASS", "root"),
			common.GetEnvOrDefaultString("SURREALDB_NAMESPACE", "open_quay"),
			common.GetEnvOrDefaultString("SURREALDB_DATABASE", "open_notebook"),
		)
	})
	return ctrl.surrealClient
}

func (ctrl *CapabilityExecuteController) Execute(c *gin.Context) {
	capabilityID := c.Param("capability_id")
	userID := c.GetString("id")

	var envelope struct {
		RequestID    string                 `json:"request_id"`
		CapabilityID string                 `json:"capability_id"`
		Input        map[string]interface{} `json:"input"`
	}
	if err := c.ShouldBindJSON(&envelope); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid envelope structure"})
		return
	}

	inputBytes, _ := json.Marshal(envelope.Input)

	requestID := envelope.RequestID
	if requestID == "" {
		requestID = "req-" + time.Now().Format("20060102150405")
	}

	switch capabilityID {
	case "PPT":
		var input service.PptExecuteInput
		if err := json.Unmarshal(inputBytes, &input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input for PPT"})
			return
		}
		bundle, err := ctrl.assembleGroundingWithContext(c, input.NotebookID, input.SourceRefs)
		if err != nil {
			c.JSON(statusFromGroundingErr(err), gin.H{"error": err.Error()})
			return
		}
		input.GroundingBundle = bundle
		quota := 1
		preConsumed, err := service.PreConsumeQuotaForJob(userID, quota)
		if err != nil {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		job, err := ctrl.pptAdapter.CreatePptJob(c.Request.Context(), requestID, userID, input)
		if err != nil {
			service.RefundQuotaForJob(userID, preConsumed)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		job.QuotaPrecheckID = strconv.Itoa(preConsumed)
		if err := common.SavePptJob(job); err != nil {
			service.RefundQuotaForJob(userID, preConsumed)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to persist job state"})
			return
		}
		c.JSON(http.StatusAccepted, gin.H{
			"request_id":    job.RequestID,
			"capability_id": "PPT",
			"job_id":        job.JobID,
			"status":        "pending",
			"status_url":    "/api/ppt/jobs/" + job.JobID,
			"created_at":    job.CreatedAt.Format(time.RFC3339),
		})

	case "PODCAST":
		var input service.PodcastExecuteInput
		if err := json.Unmarshal(inputBytes, &input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input for PODCAST"})
			return
		}
		bundle, err := ctrl.assembleGroundingWithContext(c, input.NotebookID, input.SourceRefs)
		if err != nil {
			c.JSON(statusFromGroundingErr(err), gin.H{"error": err.Error()})
			return
		}
		input.GroundingBundle = bundle
		quota := int(math.Ceil(float64(input.MaxDurationSecs) / 60))
		if quota == 0 {
			quota = 1
		}
		preConsumed, err := service.PreConsumeQuotaForJob(userID, quota)
		if err != nil {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		job, err := ctrl.podcastAdapter.CreatePodcastJob(c.Request.Context(), requestID, userID, input)
		if err != nil {
			service.RefundQuotaForJob(userID, preConsumed)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		job.QuotaPrecheckID = strconv.Itoa(preConsumed)
		if err := common.SavePodcastJob(job); err != nil {
			service.RefundQuotaForJob(userID, preConsumed)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to persist job state"})
			return
		}
		c.JSON(http.StatusAccepted, gin.H{
			"request_id":    job.RequestID,
			"capability_id": "PODCAST",
			"job_id":        job.JobID,
			"status":        "pending",
			"status_url":    "/api/podcast/jobs/" + job.JobID,
			"created_at":    job.CreatedAt.Format(time.RFC3339),
		})

	default:
		c.JSON(http.StatusNotFound, gin.H{"error": "Capability not supported"})
	}
}

func (ctrl *CapabilityExecuteController) assembleGroundingWithContext(c *gin.Context, notebookID string, sourceRefs []string) (*service.GroundingBundle, error) {
	if notebookID == "" {
		return nil, nil
	}
	dbClient := ctrl.getSurrealClient()
	if dbClient == nil {
		return nil, ctrl.surrealErr
	}
	assembler := service.NewGroundingAssembler(dbClient)
	return assembler.AssembleGroundingBundle(c.Request.Context(), service.AssembleInput{
		NotebookID: notebookID,
		SourceRefs: sourceRefs,
	})
}

func statusFromGroundingErr(err error) int {
	switch err {
	case service.ErrNotebookNotFound, service.ErrSourcesNotFound:
		return http.StatusNotFound
	case service.ErrWorkspaceUnavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}
