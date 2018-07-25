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
	TaskID       string
	TargetID     string
	Stage        uint8
}

type Response struct {
	Command     string
	Stdout      string
	Stderr      string
	LineNum     uint32
	TimeElapsed float64
	//TimeRemaining float64
}
