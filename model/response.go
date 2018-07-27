package model

type ResponseType string

const (
	ResponseUnspecified   ResponseType = "RES"
	ResponseAck                        = "ACK"          // received task announcement
	ResponseAckTask                    = "ACK_TASK"     // received task
	ResponseAckTransfer                = "ACK_TRANSFER" // completed transfer to local file system
	ResponseLog                        = "LOG"          // response stdout and stderr
	ResponseError                      = "ERROR"        // task ended with errors
	ResponseSuccess                    = "SUCCESS"      // task ended without errors
	ResponseClientError                = "CLIENT_ERROR" // client errors
	ResponseAdvertisement              = "ADV"          // agent advertisement
	ResponseRunnerLog                  = "RUNLOG"       // runner stdout and stderr
)

type BatchResponse struct {
	ResponseType ResponseType
	Responses    []Response
	TimeElapsed  float64
	Stage        StageType
	// identifiers
	TaskID   string
	TargetID string
}

type Response struct {
	Command     string `json:",omitempty"'`
	Output      string
	Error       bool
	LineNum     uint32  `json:",omitempty"'`
	TimeElapsed float64 `json:",omitempty"'`
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
