package controller

import (
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

// InternalQuotaController handles internal quota operations.
type InternalQuotaController struct{}

func NewInternalQuotaController() *InternalQuotaController {
	return &InternalQuotaController{}
}

// CreateReservationRequest matches the contract schema.
type CreateReservationRequest struct {
	RequestID      string         `json:"request_id" binding:"required"`
	IdempotencyKey string         `json:"idempotency_key" binding:"required"`
	UserID         string         `json:"user_id" binding:"required"`
	WorkspaceID    string         `json:"workspace_id"`
	CapabilityID   string         `json:"capability_id" binding:"required"`
	EstimatedUnits map[string]int `json:"estimated_units" binding:"required"`
}

// ReservationResponse matches the contract schema.
type ReservationResponse struct {
	ReservationID  string `json:"reservation_id"`
	Status         string `json:"status"`
	ReservedUnits  int64  `json:"reserved_units"`
	ExpiresAt      string `json:"expires_at"`
}

// SettleReservationRequest matches the contract schema.
type SettleReservationRequest struct {
	JobID        string         `json:"job_id" binding:"required"`
	Status       string         `json:"status" binding:"required"`
	ActualUnits  map[string]int `json:"actual_units" binding:"required"`
	ArtifactRefs []string       `json:"artifact_refs"`
}

// ReleaseReservationRequest matches the contract schema.
type ReleaseReservationRequest struct {
	JobID  string `json:"job_id" binding:"required"`
	Reason string `json:"reason" binding:"required"`
}

// sumUnits sums all values in a UnitMap.
func sumUnits(m map[string]int) int64 {
	var total int64
	for _, v := range m {
		total += int64(v)
	}
	return total
}

// CreateReservation reserves quota.
func (ctrl *InternalQuotaController) CreateReservation(c *gin.Context) {
	var req CreateReservationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, err := strconv.Atoi(req.UserID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user_id"})
		return
	}

	estimatedUnits := sumUnits(req.EstimatedUnits)

	input := service.CreateReservationInput{
		RequestID:      req.RequestID,
		IdempotencyKey: req.IdempotencyKey,
		UserID:         userID,
		WorkspaceID:    req.WorkspaceID,
		CapabilityID:   req.CapabilityID,
		EstimatedUnits: estimatedUnits,
	}

	result, err := service.CreateReservation(input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, ReservationResponse{
		ReservationID: result.ReservationID,
		Status:        result.Status,
		ReservedUnits: result.ReservedUnits,
		ExpiresAt:     result.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}

// SettleReservation settles actual usage.
func (ctrl *InternalQuotaController) SettleReservation(c *gin.Context) {
	reservationID := c.Param("reservation_id")
	if reservationID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "reservation_id is required"})
		return
	}

	var req SettleReservationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate status enum
	if req.Status != "succeeded" && req.Status != "partially_completed" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "status must be succeeded or partially_completed"})
		return
	}

	input := service.SettleReservationInput{
		ReservationID: reservationID,
		JobID:         req.JobID,
		Status:        req.Status,
		ActualUnits:   sumUnits(req.ActualUnits),
		ArtifactRefs:  req.ArtifactRefs,
	}

	if err := service.SettleReservation(input); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "settled"})
}

// ReleaseReservation releases reserved quota.
func (ctrl *InternalQuotaController) ReleaseReservation(c *gin.Context) {
	reservationID := c.Param("reservation_id")
	if reservationID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "reservation_id is required"})
		return
	}

	var req ReleaseReservationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	input := service.ReleaseReservationInput{
		ReservationID: reservationID,
		JobID:         req.JobID,
		Reason:        req.Reason,
	}

	if err := service.ReleaseReservation(input); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "released"})
}
