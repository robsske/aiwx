// Package workflow implements workflow template execution with DAG-based orchestration.
package workflow

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// ─── Types ────────────────────────────────────────────────────────────────────

// NodeType defines the type of workflow node.
type NodeType string

const (
	NodeTypeJob       NodeType = "job"
	NodeTypeApproval  NodeType = "approval"
	NodeTypeCondition NodeType = "condition"
)

// NodeStatus defines the status of a workflow node.
type NodeStatus string

const (
	NodeStatusPending   NodeStatus = "pending"
	NodeStatusRunning   NodeStatus = "running"
	NodeStatusSuccess   NodeStatus = "success"
	NodeStatusFailed    NodeStatus = "failed"
	NodeStatusSkipped   NodeStatus = "skipped"
	NodeStatusWaiting   NodeStatus = "waiting" // waiting for approval
)

// Node represents a single node in a workflow.
type Node struct {
	ID          uuid.UUID       `json:"id"`
	Name        string          `json:"name"`
	Type        NodeType        `json:"type"`
	JobTemplate *uuid.UUID      `json:"job_template_id,omitempty"`
	Approval    *ApprovalConfig `json:"approval,omitempty"`
	Condition   *ConditionConfig `json:"condition,omitempty"`
	Status      NodeStatus      `json:"status"`
	JobID       *uuid.UUID      `json:"job_id,omitempty"`
	StartedAt   *time.Time      `json:"started_at,omitempty"`
	FinishedAt  *time.Time      `json:"finished_at,omitempty"`
	Error       string          `json:"error,omitempty"`
}

// ApprovalConfig configures an approval node.
type ApprovalConfig struct {
	RequiredApprovers int      `json:"required_approvers"`
	ApproverRoles     []string `json:"approver_roles"`
	TimeoutHours      int      `json:"timeout_hours"`
}

// ConditionConfig configures a condition node.
type ConditionConfig struct {
	Expression string `json:"expression"`
}

// Edge represents a dependency between nodes.
type Edge struct {
	From uuid.UUID `json:"from"`
	To   uuid.UUID `json:"to"`
}

// Workflow represents a workflow template.
type Workflow struct {
	ID             uuid.UUID `json:"id"`
	Name           string    `json:"name"`
	OrganizationID uuid.UUID `json:"organization_id"`
	Nodes          []Node    `json:"nodes"`
	Edges          []Edge    `json:"edges"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// WorkflowRun represents an execution of a workflow.
type WorkflowRun struct {
	ID         uuid.UUID `json:"id"`
	WorkflowID uuid.UUID `json:"workflow_id"`
	Status     string    `json:"status"`
	Nodes      []Node    `json:"nodes"`
	Edges      []Edge    `json:"edges"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
}

// ─── DAG Engine ───────────────────────────────────────────────────────────────

// Engine executes workflow DAGs.
type Engine struct {
	logger *slog.Logger
}

// NewEngine creates a new workflow engine.
func NewEngine(logger *slog.Logger) *Engine {
	return &Engine{logger: logger}
}

// Validate checks if a workflow DAG is valid.
func (e *Engine) Validate(wf *Workflow) error {
	nodeMap := make(map[uuid.UUID]*Node)
	for i := range wf.Nodes {
		nodeMap[wf.Nodes[i].ID] = &wf.Nodes[i]
	}

	// Check edges reference valid nodes
	for _, edge := range wf.Edges {
		if _, ok := nodeMap[edge.From]; !ok {
			return fmt.Errorf("edge from node %s not found", edge.From)
		}
		if _, ok := nodeMap[edge.To]; !ok {
			return fmt.Errorf("edge to node %s not found", edge.To)
		}
	}

	// Check for cycles
	if hasCycle(wf) {
		return fmt.Errorf("workflow contains a cycle")
	}

	return nil
}

// hasCycle detects cycles in the workflow DAG.
func hasCycle(wf *Workflow) bool {
	adj := make(map[uuid.UUID][]uuid.UUID)
	for _, edge := range wf.Edges {
		adj[edge.From] = append(adj[edge.From], edge.To)
	}

	visited := make(map[uuid.UUID]bool)
	recStack := make(map[uuid.UUID]bool)

	var dfs func(uuid.UUID) bool
	dfs = func(node uuid.UUID) bool {
		visited[node] = true
		recStack[node] = true

		for _, neighbor := range adj[node] {
			if !visited[neighbor] {
				if dfs(neighbor) {
					return true
				}
			} else if recStack[neighbor] {
				return true
			}
		}

		recStack[node] = false
		return false
	}

	for _, node := range wf.Nodes {
		if !visited[node.ID] {
			if dfs(node.ID) {
				return true
			}
		}
	}

	return false
}

// GetReadyNodes returns nodes that are ready to execute (all dependencies satisfied).
func (e *Engine) GetReadyNodes(wf *WorkflowRun) []Node {
	nodeMap := make(map[uuid.UUID]*Node)
	for i := range wf.Nodes {
		nodeMap[wf.Nodes[i].ID] = &wf.Nodes[i]
	}

	// Build reverse dependency map
	dependsOn := make(map[uuid.UUID][]uuid.UUID)
	for _, edge := range wf.Edges {
		dependsOn[edge.To] = append(dependsOn[edge.To], edge.From)
	}

	var ready []Node
	for i := range wf.Nodes {
		node := &wf.Nodes[i]
		if node.Status != NodeStatusPending {
			continue
		}

		// Check if all dependencies are satisfied
		allSatisfied := true
		for _, depID := range dependsOn[node.ID] {
			dep, ok := nodeMap[depID]
			if !ok {
				continue
			}
			if dep.Status == NodeStatusFailed {
				node.Status = NodeStatusSkipped
				allSatisfied = false
				break
			}
			if dep.Status != NodeStatusSuccess {
				allSatisfied = false
				break
			}
		}

		if allSatisfied {
			ready = append(ready, *node)
		}
	}

	return ready
}

// UpdateNodeStatus updates the status of a node and propagates changes.
func (e *Engine) UpdateNodeStatus(wf *WorkflowRun, nodeID uuid.UUID, status NodeStatus, errMsg string) {
	for i := range wf.Nodes {
		if wf.Nodes[i].ID == nodeID {
			wf.Nodes[i].Status = status
			now := time.Now()
			if status == NodeStatusRunning {
				wf.Nodes[i].StartedAt = &now
			}
			if status == NodeStatusSuccess || status == NodeStatusFailed || status == NodeStatusSkipped {
				wf.Nodes[i].FinishedAt = &now
			}
			wf.Nodes[i].Error = errMsg
			break
		}
	}

	// Check if workflow is complete
	e.checkCompletion(wf)
}

// checkCompletion checks if all nodes are terminal and updates workflow status.
func (e *Engine) checkCompletion(wf *WorkflowRun) {
	allTerminal := true
	anyFailed := false

	for _, node := range wf.Nodes {
		switch node.Status {
		case NodeStatusSuccess, NodeStatusSkipped:
			// OK
		case NodeStatusFailed:
			anyFailed = true
			allTerminal = true
		default:
			allTerminal = false
		}
	}

	if allTerminal {
		now := time.Now()
		wf.FinishedAt = &now
		if anyFailed {
			wf.Status = "failed"
		} else {
			wf.Status = "success"
		}
	}
}

// GetExecutionOrder returns the nodes in topological order.
func (e *Engine) GetExecutionOrder(wf *Workflow) ([]uuid.UUID, error) {
	adj := make(map[uuid.UUID][]uuid.UUID)
	inDegree := make(map[uuid.UUID]int)

	for _, node := range wf.Nodes {
		inDegree[node.ID] = 0
	}

	for _, edge := range wf.Edges {
		adj[edge.From] = append(adj[edge.From], edge.To)
		inDegree[edge.To]++
	}

	// Kahn's algorithm
	var queue []uuid.UUID
	for _, node := range wf.Nodes {
		if inDegree[node.ID] == 0 {
			queue = append(queue, node.ID)
		}
	}

	var order []uuid.UUID
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		order = append(order, node)

		for _, neighbor := range adj[node] {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor)
			}
		}
	}

	if len(order) != len(wf.Nodes) {
		return nil, fmt.Errorf("cycle detected in workflow")
	}

	return order, nil
}
