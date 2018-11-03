package model

const (
	// Request types
	RequestTargetAll = "ALL"
	RequestTargetID  = "ID"
	RequestTargetTag = "TAG"
	// Stage types
	StageAssemble    = "ASSEMBLE"
	StageTransfer    = "TRANSFER"
	StageInstall     = "INSTALL"
	StageTest        = "TEST"
	StageRun         = "RUN"
	// Other consts
	PrefixSeparator = "-"
)

// Task is a struct with all the information for deployment on a target
type Task struct {
	ID        string
	Artifacts []byte
	Install   []string
	Run       []string
	Debug     bool
}

type TaskAnnouncement struct {
	ID    string
	Size  uint64
	Debug bool
}

type LogRequest struct {
	IfModifiedSince UnixTimeType
}

func TargetTopic(id string) string {
	return RequestTargetID + PrefixSeparator + id
}

func TargetTag(tag string) string {
	return RequestTargetTag + PrefixSeparator + tag
}
