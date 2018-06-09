package model

type Task struct {
	ID        string
	Commands  []string
	Artifacts []byte
	Log       Log
	Time      int64
}

type Log struct {
	Interval  string
	Verbosity string
}
