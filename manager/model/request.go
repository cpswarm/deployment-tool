package model

const (
	// Request types (should not contain PrefixSeparator or TopicSeperator chars)
	RequestTargetAll = "ALL"
	RequestTargetID  = "ID"
	RequestTargetTag = "TAG"
	// Stage types
	StageAssemble = "ASSEMBLE"
	StageTransfer = "TRANSFER"
	StageInstall  = "INSTALL"
	StageTest     = "TEST"
	StageRun      = "RUN"
	// Other consts
	PrefixSeparator = "-"
)

type Stages struct {
	Assemble []string `json:"assemble"`
	Transfer []string `json:"transfer"`
	Install  []string `json:"install"`
	//Test     []string
	Run []string `json:"run"`
}

// Header contains information that is common among task related structs
type Header struct {
	ID      string `json:"id"`
	Debug   bool   `json:"debug"`
	Created int64  `json:"created"`
}

// Announcement carries information about a task
type Announcement struct {
	Header
	Size int `json:"size"`
}

// Task is a struct with all the information for deployment on a target
type Task struct {
	Header
	Stages    Stages `json:"stages"`
	Artifacts []byte `json:"artifacts,omitempty"`
}

type LogRequest struct {
	IfModifiedSince UnixTimeType
}

// RequestWrapper is the struct of messages sent to request topics
type RequestWrapper struct {
	Announcement *Announcement `json:"a,omitempty"`
	LogRequest   *LogRequest   `json:"l,omitempty"`
	Command      *string       `json:"c,omitempty"`
}

func FormatTopicID(id string) string {
	return RequestTargetID + PrefixSeparator + id
}

func FormatTopicTag(tag string) string {
	return RequestTargetTag + PrefixSeparator + tag
}
