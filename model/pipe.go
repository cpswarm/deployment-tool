package model

// Pipe is a bi-directional channel structure
//	for communication between the clients and manager/agent
type Pipe struct {
	RequestCh  chan Request
	ResponseCh chan BatchResponse
}

// NewPipe returns an instantiated Pipe
func NewPipe() Pipe {
	return Pipe{
		RequestCh:  make(chan Request),
		ResponseCh: make(chan BatchResponse),
	}
}
