package repository

import (
	"testing"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/cloudaccounts/repository/db"
)

// The cost-data reconciliation of the latest sync is stored on the connection row; the API must hand it on. It once did not
// (the account page said "no synchronization has been checked" over a history full of checked syncs).
func TestConnectionCarriesItsLastQuality(t *testing.T) {
	at := time.Now()
	row := db.CloudaccountsConnection{Provider: "azure", SyncStatus: "healthy", LastSyncAt: &at,
		LastQuality: []byte(`{"status":"ok","checked_at":"2026-10-11T12:00:45Z","checked_days":7,"max_diff_pct":0,"mismatch_days":0}`)}
	c := toConn(row)
	if c.LastQuality == nil || c.LastQuality.Status != "ok" || c.LastQuality.CheckedDays != 7 {
		t.Fatalf("data quality lost: %+v", c.LastQuality)
	}
	if toConn(db.CloudaccountsConnection{Provider: "azure", SyncStatus: "pending"}).LastQuality != nil {
		t.Error("a connection that was never checked must have no quality")
	}
}
