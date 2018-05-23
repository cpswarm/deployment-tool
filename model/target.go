package model

type Target struct {
	ID          string
	Description string
	Type        string
	Tasks       TargetTask
}

type TargetTask struct {
	LatestBatchResponse BatchResponse
	History             []string
}
