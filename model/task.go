package model

// Task is a struct with all the information for deployment on a target
type Task struct {
	ID         string
	Commands   []string
	Artifacts  []byte
	Log        Log
	Activation []string
}

type Log struct {
	Interval  string
	Verbosity string
}

type TaskAnnouncement struct {
	ID   string
	Size uint64
}
