package model

import "time"

const (
	// Response types (topics)
	ResponseLogs          = "LOG" // logs
	ResponseAdvertisement = "ADV" // device advertisement
	ResponsePackage       = "PKG" // assembled artifacts

	// Log output constants
	ExecStart        = "EXEC-START"
	ExecEnd          = "EXEC-END"
	StageStart       = "STAGE-START"
	StageEnd         = "STAGE-END"
	CommandByAgent   = "$agent"
	CommandByManager = "$manager"
)

type Response struct {
	TargetID  string
	Logs      []Log
	OnRequest bool `json:",omitempty"` // true when logs were requested explicitly
}

// UnixTimeType is the type used for log timestamps
type UnixTimeType int64

// UnixTime returns the current unix time
func UnixTime() UnixTimeType {
	return UnixTimeType(time.Now().UnixNano() / 1e6)
}

type Log struct {
	Task    string       `json:"task,omitempty"`
	Stage   string       `json:"stage,omitempty"`
	Command string       `json:"command,omitempty"` // TODO is command unique within a stage?
	Output  string       `json:"output,omitempty"`
	Error   bool         `json:"error,omitempty"`
	Time    UnixTimeType `json:"time,omitempty"`
	Debug   bool         `json:"-"`
}

type Location struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

type TargetBase struct {
	// omitempty for all fields to allow patch updates
	ID        string    `json:"id,omitempty"`
	Tags      []string  `json:"tags,omitempty"`
	Location  *Location `json:"location,omitempty"`
	PublicKey string    `json:"publicKey,omitempty"`
}

type Package struct {
	Assembler string `json:"a"`
	Task      string `json:"t"`
	Payload   []byte `json:"p"`
}

type ZeromqServerInfo struct {
	PublicKey string `json:"publicKey"`
	PubPort   string `json:"pubPort"`
	SubPort   string `json:"subPort"`
}

type ServerInfo struct {
	ZeroMQ ZeromqServerInfo `json:"zeromq"`
}
