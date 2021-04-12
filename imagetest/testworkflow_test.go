package imagetest

import (
	"testing"
)

func TestNewTestWorkflow(t *testing.T) {
	twf, err := NewTestWorkflow("name", "image")
	if err != nil {
		t.Errorf("failed to create test workflow: %v", err)
	}
	if twf.wf == nil {
		t.Error("test workflow is malformed")
	}
	if len(twf.wf.Steps) != 0 {
		t.Error("test workflow has initial steps")
	}
}
