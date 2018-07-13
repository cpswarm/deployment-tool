package model

// Task is a struct with all the information for deployment on a target
type Task struct {
	ID         string
	Commands   []string
	Artifacts  []byte
	Log        Log
	Activation Activation
}

type Log struct {
	Interval  string
	Verbosity string
}

type TaskAnnouncement struct {
	ID   string
	Size uint64
}



type Activation struct {
	Execute       []string
	AutoStart     bool `yaml:"autoStart" json:"autoStart"`
	RemoteControl bool
}
