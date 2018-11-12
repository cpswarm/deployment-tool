package model

import "time"

const (
	// Response types (topics)
	ResponseLog           = "LOG" // logs
	ResponseAdvertisement = "ADV" // device advertisement
	//ResponseSuccess = "SUCCESS" // stage ended without errors
	//ResponseError   = "ERROR"   // stage ended with errors
	//ResponseClientError   = "CLIENT_ERROR" // client errors

	// Log output constants
	ExecStart  = "EXEC-START"
	ExecEnd    = "EXEC-END"
	StageStart = "STAGE-START"
	StageEnd   = "STAGE-END"
)

type Response struct {
	TargetID string
	Logs     []Log
}

// UnixTimeType is the integer type used for logging timestamps. For the time being, we use uint32 i.e. good for 1970-2106
type UnixTimeType uint32

// UnixTime returns the current unix time
func UnixTime() UnixTimeType {
	return UnixTimeType(time.Now().Unix())
}

type Log struct {
	Task    string
	Stage   string
	Command string `json:",omitempty"'` // TODO is command unique within a stage?
	Output  string
	Error   bool         `json:",omitempty"'`
	Time    UnixTimeType `json:",omitempty"'`
	Debug   bool         `json:"-"`
}

type Target struct {
	// identification attributes
	ID        string   // TODO change this to alias and always generate UUID ? alias==tag ? //
	AutoGenID string   `json:",omitempty"'`
	Tags      []string `json:",omitempty"'`

	// active task
	TaskID      string   `json:",omitempty"'`
	TaskDebug   bool     `json:",omitempty"'`
	TaskRun     []string `json:",omitempty"'`
	TaskHistory []string `json:",omitempty"'`
}
