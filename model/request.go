package model

const RequestTaskAnnouncement = "TASK"

type Request struct {
	Topic   string
	Payload []byte
}
