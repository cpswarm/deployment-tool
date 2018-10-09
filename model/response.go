package model

import "time"

type ResponseType string

const (
	// Log types
	ResponseLog     ResponseType = "LOG"     // stage stdout and stderr
	ResponseSuccess ResponseType = "SUCCESS" // stage ended without errors
	ResponseError   ResponseType = "ERROR"   // stage ended with errors
	ProcessStart                 = "START"
	ProcessExit                  = "EXIT"

	ResponseClientError   ResponseType = "CLIENT_ERROR" // client errors
	ResponseAdvertisement ResponseType = "ADV"          // agent advertisement

)

type Response struct {
	Stage StageType
	Logs  []Log

	// identifiers
	TaskID   string
	TargetID string
}

// UnixTimeType is the integer type used for logging timestamps. For the time being, we use uint32 i.e. good for 1970-2106
type UnixTimeType uint32

// UnixTime returns the current unix time
func UnixTime() UnixTimeType {
	return UnixTimeType(time.Now().Unix())
}

type Log struct {
	Command string `json:",omitempty"'`
	Output  string
	Error   bool
	LineNum uint32       `json:",omitempty"'`
	Time    UnixTimeType `json:",omitempty"'`
}

type Target struct {
	// identification attributes
	ID        string // TODO change this to alias and always generate UUID ? alias==tag ? //
	AutoGenID string
	Tags      []string

	// active task
	TaskID      string
	TaskStage   StageType
	TaskStatus  ResponseType
	Debug       bool
	TaskRun     []string `json:",omitempty"'`
	TaskHistory []string `json:",omitempty"'`
}
