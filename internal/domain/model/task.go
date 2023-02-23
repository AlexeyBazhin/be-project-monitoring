package model

import (
	"database/sql"
	"time"
)

const (
	TODO       TaskStatus = "BACKLOG"
	InProgress TaskStatus = "IN_PROGRESS"
	InReview   TaskStatus = "REVIEW"
	//Testing    TaskStatus = "Testing"
	Done TaskStatus = "DONE"
)

type (
	TaskStatus string
	Task       struct {
		ShortTask
		ProjectID int `json:"projectId"`
	}
	ShortTask struct {
		ID            int            `json:"id"`
		Name          string         `json:"title"`
		Description   sql.NullString `json:"description"`
		ParticipantID sql.NullInt64  `json:"asignee"`
		CreatorID     sql.NullInt64  `json:"creatorId"`
		Status        TaskStatus     `json:"status"`
		Estimate      sql.NullString `json:"estimatedTime"`
		CreatedAt     time.Time      `json:"createdAt"`
		UpdatedAt     time.Time      `json:"updatedAt"`
	}
	TaskInfo struct {
		Task
		Creator     ShortUser
		Participant ShortUser
	}
)

var TaskStatuses = map[string]struct{}{
	"BACKLOG":     {},
	"IN_PROGRESS": {},
	"REVIEW":      {},
	//"Testing":     {},
	"DONE": {},
}
