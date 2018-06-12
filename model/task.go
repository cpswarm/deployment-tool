package model

type Task struct {
	ID           string
	Commands     []string
	Artifacts    []byte
	Log          Log
	Time         int64
	Size         uint64
	Announcement bool
}

type Log struct {
	Interval  string
	Verbosity string
}
