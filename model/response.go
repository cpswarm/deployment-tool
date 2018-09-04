package model

import "time"

type ResponseType string

const (
	// Response types
	ResponseLog     ResponseType = "LOG"     // stage stdout and stderr
	ResponseSuccess ResponseType = "SUCCESS" // stage ended without errors
	ResponseError   ResponseType = "ERROR"   // stage ended with errors

	ResponseClientError   ResponseType = "CLIENT_ERROR" // client errors
	ResponseAdvertisement ResponseType = "ADV"          // agent advertisement
)

type BatchResponse struct {
	Stage        StageType
	ResponseType ResponseType
	Responses    []Response
	//TimeElapsed  float64

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

type Response struct {
	Command string `json:",omitempty"'`
	Output  string
	Error   bool
	LineNum uint32       `json:",omitempty"'`
	Time    UnixTimeType `json:",omitempty"'`
}

type Target struct {
	// identification attributes
	ID        string
	AutoGenID string
	Tags      []string

	// active task
	TaskID      string
	TaskStage   StageType
	TaskStatus  ResponseType
	TaskRun     []string `json:",omitempty"'`
	TaskHistory []string `json:",omitempty"'`
}
