package model

type ResponseType uint8

const (
	ResponseTypeUnspecified ResponseType = iota
	ResponseTypeACK
	ResponseTypeLog
)

type BatchResponse struct {
	ResponseType ResponseType
	Responses    []Response
	TimeElapsed  float64
	TaskID       string
}

type Response struct {
	Command     string
	Stdout      string
	Stderr      string
	LineNum     uint32
	TimeElapsed float64
	//TimeRemaining float64
}
