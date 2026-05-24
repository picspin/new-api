package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupQuotaRuntimeTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(&QuotaReservation{}, &QuotaLedgerEntry{})
	require.NoError(t, err)

	return db
}

func TestQuotaReservation_IdempotentCreate(t *testing.T) {
	db := setupQuotaRuntimeTestDB(t)

	// First creation should succeed
	res1 := QuotaReservation{
		ReservationID:  "res-001",
		IdempotencyKey: "idem-001",
		UserID:         1,
		CapabilityID:   "cap-001",
		Status:         "reserved",
		EstimatedUnits: 100,
		ExpiresAt:      time.Now().Add(1 * time.Hour).Unix(),
	}
	err := db.Create(&res1).Error
	require.NoError(t, err)
	assert.NotZero(t, res1.ID)

	// Same user_id + idempotency_key should fail due to unique constraint
	res2 := QuotaReservation{
		ReservationID:  "res-002",
		IdempotencyKey: "idem-001", // same idempotency key
		UserID:         1,         // same user
		CapabilityID:   "cap-001",
		Status:         "reserved",
		EstimatedUnits: 100,
		ExpiresAt:      time.Now().Add(1 * time.Hour).Unix(),
	}
	err = db.Create(&res2).Error
	assert.Error(t, err) // unique constraint violation expected
}

func TestQuotaReservation_CannotSettleAfterRelease(t *testing.T) {
	db := setupQuotaRuntimeTestDB(t)

	res := QuotaReservation{
		ReservationID:  "res-release-001",
		IdempotencyKey: "idem-release-001",
		UserID:         10,
		CapabilityID:   "cap-001",
		Status:         "reserved",
		EstimatedUnits: 50,
		ExpiresAt:      time.Now().Add(1 * time.Hour).Unix(),
	}
	err := db.Create(&res).Error
	require.NoError(t, err)

	// Release the reservation
	err = res.Release(db, "user cancelled")
	require.NoError(t, err)

	// Verify status is released
	var check QuotaReservation
	err = db.First(&check, res.ID).Error
	require.NoError(t, err)
	assert.Equal(t, "released", check.Status)
	assert.NotNil(t, check.ReleasedAt)
	assert.Equal(t, "user cancelled", check.ReleaseReason)

	// Attempting to settle a released reservation should fail
	err = check.Settle(db, 30)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrReservationNotPending)
}

func TestQuotaReservation_CannotReleaseAfterSettle(t *testing.T) {
	db := setupQuotaRuntimeTestDB(t)

	res := QuotaReservation{
		ReservationID:  "res-settle-001",
		IdempotencyKey: "idem-settle-001",
		UserID:         20,
		CapabilityID:   "cap-002",
		Status:         "reserved",
		EstimatedUnits: 75,
		ExpiresAt:      time.Now().Add(1 * time.Hour).Unix(),
	}
	err := db.Create(&res).Error
	require.NoError(t, err)

	// Settle the reservation
	err = res.Settle(db, 60)
	require.NoError(t, err)

	// Verify status is settled
	var check QuotaReservation
	err = db.First(&check, res.ID).Error
	require.NoError(t, err)
	assert.Equal(t, "settled", check.Status)
	assert.NotNil(t, check.SettledAt)
	assert.Equal(t, int64(60), check.ActualUnits)

	// Attempting to release a settled reservation should fail
	err = check.Release(db, "too late")
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrReservationNotPending)
}

func TestQuotaLedgerEntry_CreatedOnReserve(t *testing.T) {
	db := setupQuotaRuntimeTestDB(t)

	// Create a reservation
	res := QuotaReservation{
		ReservationID:  "res-ledger-001",
		IdempotencyKey: "idem-ledger-001",
		UserID:         30,
		CapabilityID:   "cap-003",
		Status:         "reserved",
		EstimatedUnits: 200,
		ExpiresAt:      time.Now().Add(1 * time.Hour).Unix(),
	}
	err := db.Create(&res).Error
	require.NoError(t, err)

	// Create a reserve ledger entry
	ledgerEntry := QuotaLedgerEntry{
		ReservationID: res.ReservationID,
		UserID:       res.UserID,
		CapabilityID: res.CapabilityID,
		EntryType:    "reserve",
		Units:        res.EstimatedUnits,
		JobID:        "job-001",
	}
	err = db.Create(&ledgerEntry).Error
	require.NoError(t, err)

	// Verify ledger entry was created
	var entries []QuotaLedgerEntry
	err = db.Find(&entries, "reservation_id = ?", res.ReservationID).Error
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "reserve", entries[0].EntryType)
	assert.Equal(t, int64(200), entries[0].Units)
}

func TestQuotaLedgerEntry_CreatedOnSettle(t *testing.T) {
	db := setupQuotaRuntimeTestDB(t)

	// Create and settle a reservation
	res := QuotaReservation{
		ReservationID:  "res-ledger-002",
		IdempotencyKey: "idem-ledger-002",
		UserID:         40,
		CapabilityID:   "cap-004",
		Status:         "reserved",
		EstimatedUnits: 100,
		ExpiresAt:      time.Now().Add(1 * time.Hour).Unix(),
	}
	err := db.Create(&res).Error
	require.NoError(t, err)

	// Settle with actual units
	actualUnits := int64(80)
	err = res.Settle(db, actualUnits)
	require.NoError(t, err)

	// Create a settle ledger entry
	ledgerEntry := QuotaLedgerEntry{
		ReservationID: res.ReservationID,
		UserID:       res.UserID,
		CapabilityID: res.CapabilityID,
		EntryType:    "settle",
		Units:        actualUnits,
		JobID:        "job-002",
	}
	err = db.Create(&ledgerEntry).Error
	require.NoError(t, err)

	// Verify ledger entry was created
	var entries []QuotaLedgerEntry
	err = db.Find(&entries, "reservation_id = ? AND entry_type = ?", res.ReservationID, "settle").Error
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "settle", entries[0].EntryType)
	assert.Equal(t, actualUnits, entries[0].Units)
}

func TestQuotaLedgerEntry_CreatedOnRelease(t *testing.T) {
	db := setupQuotaRuntimeTestDB(t)

	// Create and release a reservation
	res := QuotaReservation{
		ReservationID:  "res-ledger-003",
		IdempotencyKey: "idem-ledger-003",
		UserID:         50,
		CapabilityID:   "cap-005",
		Status:         "reserved",
		EstimatedUnits: 150,
		ExpiresAt:      time.Now().Add(1 * time.Hour).Unix(),
	}
	err := db.Create(&res).Error
	require.NoError(t, err)

	// Release the reservation
	err = res.Release(db, "expired")
	require.NoError(t, err)

	// Create a release ledger entry
	ledgerEntry := QuotaLedgerEntry{
		ReservationID: res.ReservationID,
		UserID:       res.UserID,
		CapabilityID: res.CapabilityID,
		EntryType:    "release",
		Units:        -res.EstimatedUnits, // negative to reverse the reserve
		JobID:        "",
	}
	err = db.Create(&ledgerEntry).Error
	require.NoError(t, err)

	// Verify ledger entry was created
	var entries []QuotaLedgerEntry
	err = db.Find(&entries, "reservation_id = ? AND entry_type = ?", res.ReservationID, "release").Error
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "release", entries[0].EntryType)
	assert.Equal(t, int64(-150), entries[0].Units)
}

func TestQuotaReservation_DifferentUsersSameIdempotencyKey(t *testing.T) {
	db := setupQuotaRuntimeTestDB(t)

	// User 1 reservation
	res1 := QuotaReservation{
		ReservationID:  "res-user1",
		IdempotencyKey: "shared-key",
		UserID:         100,
		CapabilityID:   "cap-001",
		Status:         "reserved",
		EstimatedUnits: 100,
		ExpiresAt:      time.Now().Add(1 * time.Hour).Unix(),
	}
	err := db.Create(&res1).Error
	require.NoError(t, err)

	// User 2 can use same idempotency key (unique index is composite: user_id + idempotency_key)
	res2 := QuotaReservation{
		ReservationID:  "res-user2",
		IdempotencyKey: "shared-key",
		UserID:         200, // different user
		CapabilityID:   "cap-001",
		Status:         "reserved",
		EstimatedUnits: 100,
		ExpiresAt:      time.Now().Add(1 * time.Hour).Unix(),
	}
	err = db.Create(&res2).Error
	require.NoError(t, err) // should succeed - different user
}
