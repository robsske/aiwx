package workflow

import (
	"testing"

	"github.com/google/uuid"
)

func TestValidate_SimpleDAG(t *testing.T) {
	engine := &Engine{}

	node1 := Node{ID: uuid.New(), Name: "Start", Type: NodeTypeJob, Status: NodeStatusPending}
	node2 := Node{ID: uuid.New(), Name: "End", Type: NodeTypeJob, Status: NodeStatusPending}

	wf := &Workflow{
		ID:    uuid.New(),
		Name:  "test",
		Nodes: []Node{node1, node2},
		Edges: []Edge{{From: node1.ID, To: node2.ID}},
	}

	if err := engine.Validate(wf); err != nil {
		t.Errorf("Validate failed: %v", err)
	}
}

func TestValidate_CycleDetection(t *testing.T) {
	engine := &Engine{}

	node1 := Node{ID: uuid.New(), Name: "A", Type: NodeTypeJob}
	node2 := Node{ID: uuid.New(), Name: "B", Type: NodeTypeJob}
	node3 := Node{ID: uuid.New(), Name: "C", Type: NodeTypeJob}

	wf := &Workflow{
		ID:    uuid.New(),
		Name:  "test",
		Nodes: []Node{node1, node2, node3},
		Edges: []Edge{
			{From: node1.ID, To: node2.ID},
			{From: node2.ID, To: node3.ID},
			{From: node3.ID, To: node1.ID}, // Cycle!
		},
	}

	if err := engine.Validate(wf); err == nil {
		t.Error("Validate should detect cycle")
	}
}

func TestGetReadyNodes(t *testing.T) {
	engine := &Engine{}

	node1 := Node{ID: uuid.New(), Name: "Start", Type: NodeTypeJob, Status: NodeStatusSuccess}
	node2 := Node{ID: uuid.New(), Name: "Middle", Type: NodeTypeJob, Status: NodeStatusPending}
	node3 := Node{ID: uuid.New(), Name: "End", Type: NodeTypeJob, Status: NodeStatusPending}

	wf := &WorkflowRun{
		ID:    uuid.New(),
		Nodes: []Node{node1, node2, node3},
		Edges: []Edge{
			{From: node1.ID, To: node2.ID},
			{From: node2.ID, To: node3.ID},
		},
	}

	ready := engine.GetReadyNodes(wf)

	// Only node2 should be ready (node1 is done, node3 depends on node2)
	if len(ready) != 1 {
		t.Errorf("ready nodes = %d, want 1", len(ready))
	}
	if ready[0].ID != node2.ID {
		t.Errorf("ready node = %s, want %s", ready[0].ID, node2.ID)
	}
}

func TestGetExecutionOrder(t *testing.T) {
	engine := &Engine{}

	node1 := Node{ID: uuid.New(), Name: "A"}
	node2 := Node{ID: uuid.New(), Name: "B"}
	node3 := Node{ID: uuid.New(), Name: "C"}

	wf := &Workflow{
		ID:    uuid.New(),
		Nodes: []Node{node1, node2, node3},
		Edges: []Edge{
			{From: node1.ID, To: node2.ID},
			{From: node2.ID, To: node3.ID},
		},
	}

	order, err := engine.GetExecutionOrder(wf)
	if err != nil {
		t.Fatalf("GetExecutionOrder failed: %v", err)
	}

	if len(order) != 3 {
		t.Errorf("order length = %d, want 3", len(order))
	}
	if order[0] != node1.ID || order[1] != node2.ID || order[2] != node3.ID {
		t.Errorf("wrong order: %v", order)
	}
}

func TestUpdateNodeStatus_Completion(t *testing.T) {
	engine := &Engine{}

	node1 := Node{ID: uuid.New(), Name: "A", Status: NodeStatusSuccess}
	node2 := Node{ID: uuid.New(), Name: "B", Status: NodeStatusRunning}

	wf := &WorkflowRun{
		ID:    uuid.New(),
		Nodes: []Node{node1, node2},
	}

	engine.UpdateNodeStatus(wf, node2.ID, NodeStatusSuccess, "")

	if wf.Status != "success" {
		t.Errorf("workflow status = %q, want %q", wf.Status, "success")
	}
	if wf.FinishedAt == nil {
		t.Error("FinishedAt should be set")
	}
}
