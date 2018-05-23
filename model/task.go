package model

type Task struct {
	ID        string
	Commands  []string
	Artifacts []byte
	Time      int64
}
