package model

const (
	// Request types
	RequestTargetAll = "ALL"
	RequestTargetID  = "ID"
	RequestTargetTag = "TAG"
	// Stage types
	StageUnspecified StageType = ""
	StageAssemble              = "ASSEMBLE"
	StageTransfer              = "TRANSFER"
	StageInstall               = "INSTALL"
	StageTest                  = "TEST"
	StageRun                   = "RUN"
	// Other consts
	PrefixSeperator = "-"
	DefaultLogInterval = 3 * time.Second
)

type StageType string

// Task is a struct with all the information for deployment on a target
type Task struct {
	ID        string
	Artifacts []byte
	Install   []string
	Run       []string
	Log       Log
}

type Log struct {
	Interval string
	Level    string
}

func (l Log) GetInterval() time.Duration {
	interval, err := time.ParseDuration(l.Interval)
	if err != nil {
		return DefaultLogInterval
	}
	return interval
}

type TaskAnnouncement struct {
	ID   string
	Size uint64
}

type LogRequest struct {
	Stage StageType
}

func TargetTopic(id string) string {
	return RequestTargetID + PrefixSeperator + id
}

func TargetTag(tag string) string {
	return RequestTargetTag + PrefixSeperator + tag
}
