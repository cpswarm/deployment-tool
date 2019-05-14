package model

const (
	TopicSeperator   = ":"
	PipeConnected    = "CONN"
	PipeDisconnected = "DISCONN"
	// Operation
	OperationSubscribe = iota
	OperationUnsubscribe
)

// Pipe is a bi-directional channel structure
//	for communication between the clients and manager/agent
type Pipe struct {
	RequestCh   chan Message
	ResponseCh  chan Message
	OperationCh chan Operation
}

// NewPipe returns an instantiated Pipe
func NewPipe() Pipe {
	return Pipe{
		RequestCh:   make(chan Message),
		ResponseCh:  make(chan Message),
		OperationCh: make(chan Operation),
	}
}

type Message struct {
	Topic   string
	Payload []byte
}

type Operation struct {
	Type int
	Body interface{}
}
