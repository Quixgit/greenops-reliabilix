package domain

import "testing"

func TestWorkflow(t *testing.T) {
	r := &Recommendation{Status: Open, ComplianceOK: true}
	if err := r.Transition(Applied); err == nil {
		t.Fatal("open -> applied must be rejected (approval required)")
	}
	if err := r.Transition(Approved); err != nil {
		t.Fatal(err)
	}
	if err := r.Transition(Applied); err != nil {
		t.Fatal(err)
	}
	if err := r.Transition(Open); err == nil {
		t.Fatal("applied is terminal")
	}
	blocked := &Recommendation{Status: Open, ComplianceOK: false}
	if err := blocked.Transition(Approved); err == nil {
		t.Fatal("approval without compliance must be rejected")
	}
}
