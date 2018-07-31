package model

type ResponseType string

const (
	// Response types
	ResponseLog     ResponseType = "LOG"     // stage stdout and stderr
	ResponseSuccess              = "SUCCESS" // stage ended without errors
	ResponseError                = "ERROR"   // stage ended with errors

	ResponseClientError   = "CLIENT_ERROR" // client errors
	ResponseAdvertisement = "ADV"          // agent advertisement
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

// UnixTime is the integer type used for logging timestamps. For the time being, we use uint32 i.e. good for 1970-2106
type UnixTime uint32
type Response struct {
	Command string `json:",omitempty"'`
	Output  string
	Error   bool
	LineNum uint32   `json:",omitempty"'`
	Time    UnixTime `json:",omitempty"'`
}

type Target struct {
	// identification attributes
	ID   string
	Tags []string

	Tasks *TaskHistory
}

type TaskHistory struct {
	LatestBatchResponse BatchResponse
	Run                 []string
	Logging             Log
	History             []string
}
