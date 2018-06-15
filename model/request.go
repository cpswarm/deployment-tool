package model

const (
	RequestTaskAnnouncement    = "TASK"
	RequestTargetAdvertisement = "ADV"
)

type Request struct {
	Topic   string
	Payload []byte
}
