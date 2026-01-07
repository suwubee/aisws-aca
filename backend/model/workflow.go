package model

import "time"

const (
	WorkflowNodeTypeServer    = "server"
	WorkflowNodeTypeTask      = "task"
	WorkflowNodeTypeCommand   = "command"
	WorkflowNodeTypeAI        = "ai"
	WorkflowNodeTypeCondition = "condition"
)

// Workflow represents a workflow definition for automation.
type Workflow struct {
	ID          string    `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"not null" json:"name"`
	Description string    `json:"description"`
	Nodes       string    `gorm:"type:text" json:"nodes"` // JSON
	Edges       string    `gorm:"type:text" json:"edges"` // JSON
	Status      string    `gorm:"default:draft;index" json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// WorkflowNode represents a node instance inside a workflow.
type WorkflowNode struct {
	ID         string  `gorm:"primaryKey" json:"id"`
	WorkflowID string  `gorm:"not null;index" json:"workflow_id"`
	Type       string  `gorm:"not null;index" json:"type"` // server, task, command, ai, condition
	Name       string  `json:"name"`
	Config     string  `gorm:"type:text" json:"config"` // JSON
	ServerID   *string `gorm:"index" json:"server_id"`
	PositionX  float64 `json:"position_x"`
	PositionY  float64 `json:"position_y"`
}

// WorkflowRun represents one execution of a workflow.
type WorkflowRun struct {
	ID            string     `gorm:"primaryKey" json:"id"`
	WorkflowID    string     `gorm:"not null;index" json:"workflow_id"`
	Status        string     `gorm:"default:pending;index" json:"status"`
	CurrentNodeID *string    `gorm:"index" json:"current_node_id"`
	Logs          string     `gorm:"type:text" json:"logs"` // JSON
	StartedAt     *time.Time `json:"started_at"`
	CompletedAt   *time.Time `json:"completed_at"`
}
