package model

type Target struct {
	// identification attributes
	ID   string
	Tags []string

	Tasks *TaskHistory
}
