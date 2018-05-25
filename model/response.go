package model

type ResponseType string

const (
	ResponseUnspecified ResponseType = ""
	ResponseACK                      = "ACK"
	ResponseLog                      = "LOG"
	ResponseError                    = "ERROR"
	ResponseFinal                    = "FINAL"
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
