package model

type ResponseType string

const (
	ResponseUnspecified ResponseType = ""
	ResponseAck                      = "ACK"          // received task
	ResponseAckTransfer              = "ACK_TRANSFER" // completed transfer to local file system
	ResponseLog                      = "LOG"          // response stdout and stderr
	ResponseError                    = "ERROR"        // task ended with errors
	ResponseSuccess                  = "SUCCESS"      // task ended without errors
	ResponseClientError              = "CLIENT_ERROR" // client errors
)

type BatchResponse struct {
	ResponseType ResponseType
	Responses    []Response
	TimeElapsed  float64
	TaskID       string
	TargetID     string
}

type Response struct {
	Command     string
	Stdout      string
	Stderr      string
	LineNum     uint32
	TimeElapsed float64
	//TimeRemaining float64
}

//
//func (br *BatchResponse) Flush(){
//	br.Responses = []Response{}
//
//}
