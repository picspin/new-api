package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// QuotaReservationStatus constants — values must match DB enum quota_reservation_status
const (
	QuotaStatusReserved  = "reserved"
	QuotaStatusSettled    = "settled"
	QuotaStatusReleased   = "released"
)

// QuotaLedgerEntryType constants
const (
	LedgerEntryTypeReserve   = "reserve"
	LedgerEntryTypeSettle     = "settle"
	LedgerEntryTypeRelease    = "release"
	LedgerEntryTypeExpire     = "expire"
)

type QuotaReservation struct {
	ID             uint64    `gorm:"primaryKey"`
	ReservationID  string    `gorm:"uniqueIndex;size:64;not null"`
	RequestID      string    `gorm:"index;size:64;not null"`
	IdempotencyKey string    `gorm:"uniqueIndex:idx_user_idempotency;size:128;not null"`
	UserID         int       `gorm:"index;uniqueIndex:idx_user_idempotency"`
	WorkspaceID    string    `gorm:"index;size:64"`
	CapabilityID   string    `gorm:"index;size:64"`
	Status         string    `gorm:"size:16;not null;default:'pending'"`
	EstimatedUnits int64     `gorm:"not null"`
	ActualUnits    int64     `gorm:"default:0"`
	ExpiresAt      int64     `gorm:"index"`
	SettledAt      *int64
	ReleasedAt     *int64
	ReleaseReason  string    `gorm:"size:255"`
	CreatedAt      int64     `gorm:"autoCreateTime"`
	UpdatedAt      int64     `gorm:"autoUpdateTime"`
}

// QuotaLedgerEntry records quota changes for audit and reconciliation.
// ArtifactRefs stores JSON array of artifact identifiers related to the entry.
type QuotaLedgerEntry struct {
	ID            uint64 `gorm:"primaryKey"`
	ReservationID string `gorm:"index;size:64"`
	UserID        int    `gorm:"index"`
	CapabilityID  string `gorm:"index;size:64"`
	EntryType     string `gorm:"size:16;not null"`
	Units         int64  `gorm:"not null"`
	JobID         string `gorm:"index;size:64"`
	ArtifactRefs  string `gorm:"type:json"` // JSON array of artifact strings
	CreatedAt     int64  `gorm:"autoCreateTime"`
}

// ArtifactRefsSlice is a helper for serializing/deserializing JSON artifact refs
type ArtifactRefsSlice []string

func (a *ArtifactRefsSlice) Scan(value interface{}) error {
	if value == nil {
		*a = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("failed to scan ArtifactRefsSlice: expected []byte, got %T", value)
	}
	if len(bytes) == 0 {
		*a = nil
		return nil
	}
	return json.Unmarshal(bytes, a)
}

func (a ArtifactRefsSlice) Value() (interface{}, error) {
	if a == nil {
		return nil, nil
	}
	return json.Marshal(a)
}

// ErrReservationNotPending is returned when a state transition is attempted on a non-pending reservation
var ErrReservationNotPending = errors.New("reservation is not in pending status")

// ErrReservationAlreadySettled is returned when attempting to settle an already settled reservation
var ErrReservationAlreadySettled = errors.New("reservation already settled")

// ErrReservationAlreadyReleased is returned when attempting to release an already released reservation
var ErrReservationAlreadyReleased = errors.New("reservation already released")

// ErrReservationNotFound is returned when a reservation is not found
var ErrReservationNotFound = errors.New("reservation not found")

// Settle transitions a pending reservation to settled status with the actual units consumed.
func (q *QuotaReservation) Settle(db *gorm.DB, actualUnits int64) error {
	if q.Status != QuotaStatusReserved {
		if q.Status == QuotaStatusSettled {
			return ErrReservationAlreadySettled
		}
		return ErrReservationNotPending
	}
	now := time.Now().Unix()
	return db.Model(q).Updates(map[string]interface{}{
		"status":       QuotaStatusSettled,
		"actual_units": actualUnits,
		"settled_at":   &now,
	}).Error
}

// Release transitions a pending reservation to released status.
func (q *QuotaReservation) Release(db *gorm.DB, reason string) error {
	if q.Status != QuotaStatusReserved {
		if q.Status == QuotaStatusReleased {
			return ErrReservationAlreadyReleased
		}
		return ErrReservationNotPending
	}
	now := time.Now().Unix()
	return db.Model(q).Updates(map[string]interface{}{
		"status":         QuotaStatusReleased,
		"released_at":    &now,
		"release_reason": reason,
	}).Error
}

// IsPending returns true if the reservation is in reserved (active) status and can be settled or released.
func (q *QuotaReservation) IsPending() bool {
	return q.Status == QuotaStatusReserved
}

// IsSettled returns true if the reservation is in settled status
func (q *QuotaReservation) IsSettled() bool {
	return q.Status == QuotaStatusSettled
}

// IsReleased returns true if the reservation is in released status
func (q *QuotaReservation) IsReleased() bool {
	return q.Status == QuotaStatusReleased
}
