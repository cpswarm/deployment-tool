package model

// Pipe is a bi-directional channel structure
//	for communication between the clients and manager/agent
type Pipe struct {
	TaskCh     chan Task
	ResponseCh chan BatchResponse
}

// NewPipe returns an instantiated Pipe
func NewPipe() Pipe {
	return Pipe{
		TaskCh:     make(chan Task),
		ResponseCh: make(chan BatchResponse),
	}
}
