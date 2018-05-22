package model

type Target struct {
	ID                string
	Description       string
	Type              string
	TaskHistory       []string
	CurrentTask       string
	CurrentTaskStatus ResponseType
}
