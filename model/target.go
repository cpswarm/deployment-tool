package model

// TODO cannot generalize, separate into agent and manager structs

type Target struct {
	// identification attributes
	ID   string
	Tags []string

	Tasks *TaskHistory
}

type TaskHistory struct {
	LatestBatchResponse BatchResponse
	Run                 []string
	Logging             Log
	History             []string
}
