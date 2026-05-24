package service

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"

	"github.com/google/uuid"
)

// UnitMap represents a map of unit names to counts.
type UnitMap map[string]int64

// CreateReservationInput holds the input for creating a reservation.
type CreateReservationInput struct {
	RequestID      string
	IdempotencyKey string
	UserID         int
	WorkspaceID    string
	CapabilityID   string
	EstimatedUnits int64
}

// ReservationOutput holds the output of a reservation operation.
type ReservationOutput struct {
	ReservationID  string
	Status         string
	ReservedUnits  int64
	ExpiresAt      time.Time
}

// CreateReservation creates a new quota reservation or returns existing one for idempotency.
func CreateReservation(input CreateReservationInput) (*ReservationOutput, error) {
	// Check for existing reservation with same idempotency key for this user
	var existing model.QuotaReservation
	err := model.DB.Where("user_id = ? AND idempotency_key = ?", input.UserID, input.IdempotencyKey).First(&existing).Error
	if err == nil {
		// Found existing reservation - return it (idempotent)
		expiresAt := time.Unix(existing.ExpiresAt, 0)
		return &ReservationOutput{
			ReservationID: existing.ReservationID,
			Status:        existing.Status,
			ReservedUnits: existing.EstimatedUnits,
			ExpiresAt:     expiresAt,
		}, nil
	}

	// Create new reservation
	reservationID := uuid.New().String()
	expiresAt := time.Now().Add(30 * time.Minute) // 30 minute default expiration

	reservation := model.QuotaReservation{
		ReservationID:  reservationID,
		RequestID:      input.RequestID,
		IdempotencyKey: input.IdempotencyKey,
		UserID:         input.UserID,
		WorkspaceID:    input.WorkspaceID,
		CapabilityID:   input.CapabilityID,
		Status:         "reserved",
		EstimatedUnits: input.EstimatedUnits,
		ExpiresAt:      expiresAt.Unix(),
	}

	if err := model.DB.Create(&reservation).Error; err != nil {
		// Race condition: another request created the same reservation
		// between our check and insert — recover by re-querying
		if isUniqueConstraintError(err) {
			existing2, retryErr := getReservationByIdempotencyKey(input.UserID, input.IdempotencyKey)
			if retryErr != nil {
				return nil, fmt.Errorf("idempotency race recovery failed: %w", retryErr)
			}
			expiresAt2 := time.Unix(existing2.ExpiresAt, 0)
			return &ReservationOutput{
				ReservationID: existing2.ReservationID,
				Status:        existing2.Status,
				ReservedUnits: existing2.EstimatedUnits,
				ExpiresAt:     expiresAt2,
			}, nil
		}
		return nil, fmt.Errorf("failed to create reservation: %w", err)
	}

	return &ReservationOutput{
		ReservationID:  reservationID,
		Status:         "reserved",
		ReservedUnits:  input.EstimatedUnits,
		ExpiresAt:      expiresAt,
	}, nil
}

// getReservationByIdempotencyKey retrieves a reservation by user_id and idempotency_key.
func getReservationByIdempotencyKey(userID int, idempotencyKey string) (*model.QuotaReservation, error) {
	var reservation model.QuotaReservation
	err := model.DB.Where("user_id = ? AND idempotency_key = ?", userID, idempotencyKey).First(&reservation).Error
	if err != nil {
		return nil, err
	}
	return &reservation, nil
}

// isUniqueConstraintError returns true if the error is a database unique constraint violation.
func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return contains(errStr, "duplicate key") ||
		contains(errStr, "UNIQUE constraint failed") ||
		contains(errStr, "Duplicate entry") ||
		contains(errStr, "ER_DUP_ENTRY")
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// SettleReservationInput holds the input for settling a reservation.
type SettleReservationInput struct {
	ReservationID string
	JobID         string
	Status        string
	ActualUnits   int64
	ArtifactRefs  []string
}

// SettleReservation settles a reservation with actual usage.
func SettleReservation(input SettleReservationInput) error {
	return model.DB.Transaction(func(tx *gorm.DB) error {
		var reservation model.QuotaReservation
		if err := tx.Where("reservation_id = ?", input.ReservationID).First(&reservation).Error; err != nil {
			return fmt.Errorf("reservation not found: %w", err)
		}

		if err := reservation.Settle(tx, input.ActualUnits); err != nil {
			return err
		}

		// Refresh to get updated state
		if err := tx.Where("reservation_id = ?", input.ReservationID).First(&reservation).Error; err != nil {
			return fmt.Errorf("failed to refresh reservation: %w", err)
		}

		// Create ledger entry for the settlement
		entry := model.QuotaLedgerEntry{
			ReservationID: input.ReservationID,
			UserID:         reservation.UserID,
			CapabilityID:   reservation.CapabilityID,
			EntryType:     model.LedgerEntryTypeSettle,
			Units:          input.ActualUnits,
			JobID:          input.JobID,
		}
		if len(input.ArtifactRefs) > 0 {
			artifactJSON, _ := json.Marshal(input.ArtifactRefs)
			entry.ArtifactRefs = string(artifactJSON)
		}
		if err := tx.Create(&entry).Error; err != nil {
			return fmt.Errorf("failed to create settle ledger entry: %w", err)
		}

		return nil
	})
}

// ReleaseReservationInput holds the input for releasing a reservation.
type ReleaseReservationInput struct {
	ReservationID string
	JobID         string
	Reason        string
}

// ReleaseReservation releases a reserved quota.
func ReleaseReservation(input ReleaseReservationInput) error {
	return model.DB.Transaction(func(tx *gorm.DB) error {
		var reservation model.QuotaReservation
		if err := tx.Where("reservation_id = ?", input.ReservationID).First(&reservation).Error; err != nil {
			return fmt.Errorf("reservation not found: %w", err)
		}

		if err := reservation.Release(tx, input.Reason); err != nil {
			return err
		}

		// Refresh to get updated state
		if err := tx.Where("reservation_id = ?", input.ReservationID).First(&reservation).Error; err != nil {
			return fmt.Errorf("failed to refresh reservation: %w", err)
		}

		// Create ledger entry for the release: negative estimated_units to reverse the reserve
		entry := model.QuotaLedgerEntry{
			ReservationID: input.ReservationID,
			UserID:         reservation.UserID,
			CapabilityID:   reservation.CapabilityID,
			EntryType:     model.LedgerEntryTypeRelease,
			Units:          -reservation.EstimatedUnits,
			JobID:          input.JobID,
		}
		if err := tx.Create(&entry).Error; err != nil {
			return fmt.Errorf("failed to create release ledger entry: %w", err)
		}

		return nil
	})
}
