package domain

import (
	"testing"
	"time"
)

func TestCanExecute(t *testing.T) {
	if (Plan{}).CanExecute() == nil {
		t.Fatal("unapproved plan executable")
	}
	now := time.Now()
	if (Plan{ApprovedBy: "u", ApprovedAt: &now}).CanExecute() == nil {
		t.Fatal("plan without rollback executable")
	}
	if err := (Plan{ApprovedBy: "u", ApprovedAt: &now, RollbackPlan: "revert"}).CanExecute(); err != nil {
		t.Fatal(err)
	}
}
