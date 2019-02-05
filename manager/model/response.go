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
	return UnixTimeType(time.Now().UnixNano())
}

type Log struct {
	Task    string
	Stage   string
	Command string `json:",omitempty"` // TODO is command unique within a stage?
	Output  string
	Error   bool         `json:",omitempty"`
	Time    UnixTimeType `json:",omitempty"`
	Debug   bool         `json:"-"`
}

type Target struct {
	// identification attributes
	ID        string   // TODO change this to alias and always generate UUID ? alias==tag ? //
	AutoGenID string   `json:",omitempty"`
	Tags      []string `json:",omitempty"`

	// active task
	TaskID             string   `json:",omitempty"`
	TaskDebug          bool     `json:",omitempty"`
	TaskRun            []string `json:",omitempty"`
	TaskRunAutoRestart bool     `json:",omitempty"`
	TaskHistory        []string `json:",omitempty"`
}

type Package struct {
	Assembler string `json:"a"`
	Task      string `json:"t"`
	Payload   []byte `json:"p"`
}
