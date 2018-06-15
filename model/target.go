package model

type Target struct {
	// identification attributes
	ID   string
	Type string
	Tags []string

	Tasks *TaskHistory
}
