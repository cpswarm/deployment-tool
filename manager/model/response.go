package model

import "time"

const (
	// Response types (topics)
	ResponseLog           = "LOG" // logs
	ResponseAdvertisement = "ADV" // device advertisement
	ResponsePackage       = "PKG" // assembled artifacts
	//ResponseSuccess = "SUCCESS" // stage ended without errors
	//ResponseError   = "ERROR"   // stage ended with errors
	//ResponseClientError   = "CLIENT_ERROR" // client errors

	// Log output constants
	ExecStart        = "EXEC-START"
	ExecEnd          = "EXEC-END"
	StageStart       = "STAGE-START"
	StageEnd         = "STAGE-END"
	CommandByAgent   = "$agent"
	CommandByManager = "$manager"
)

type Response struct {
	TargetID  string
	Logs      []Log
	OnRequest bool `json:",omitempty"` // true when logs were requested explicitly
}

// UnixTimeType is the type used for log timestamps
type UnixTimeType int64

// UnixTime returns the current unix time
func UnixTime() UnixTimeType {
	return UnixTimeType(time.Now().UnixNano() / 1e6)
}

type Log struct {
	Task    string       `json:"task,omitempty"`
	Stage   string       `json:"stage,omitempty"`
	Command string       `json:"command,omitempty"` // TODO is command unique within a stage?
	Output  string       `json:"output,omitempty"`
	Error   bool         `json:"error,omitempty"`
	Time    UnixTimeType `json:"time,omitempty"`
	Debug   bool         `json:"-"`
}

type LogStored struct {
	Log
	Target string `json:"target,omitempty"`
}

type TargetBase struct {
	ID   string   `json:"id"`
	Tags []string `json:"tags"`
	//Location?
}
type Target struct {
	TargetBase
	AutoGenID string `json:"autoID,omitempty"`
	// active task
	TaskID             string           `json:"taskID"`
	TaskDebug          bool             `json:"taskDebug,omitempty"`
	TaskRun            []string         `json:"taskRun,omitempty"`
	TaskRunAutoRestart bool             `json:"taskRunAutoRestart,omitempty"`
	TaskHistory        map[string]uint8 `json:"taskHistory,omitempty"`
}

type Package struct {
	Assembler string `json:"a"`
	Task      string `json:"t"`
	Payload   []byte `json:"p"`
}
