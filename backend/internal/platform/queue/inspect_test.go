package queue

import (
	"encoding/json"
	"testing"
)

// Empty lists must reach the client as [] and not null (the console validates the shape).
func TestEmptyListsEncodeAsArrays(t *testing.T) {
	b, err := json.Marshal(map[string]any{"failed": []FailedTask{}, "queues": []QueueStat{}, "schedules": []Schedule{}})
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"failed":[],"queues":[],"schedules":[]}` {
		t.Fatalf("got %s", b)
	}
}
