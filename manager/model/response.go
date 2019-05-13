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
	ID       string    `json:"id,omitempty"`
	Tags     []string  `json:"tags,omitempty"`
	Location *Location `json:"location,omitempty"`
}

type Target struct {
	TargetBase
	AutoGenID string `json:"autoID,omitempty"`
	// active task
	TaskID             string           `json:"taskID"`
	TaskDebug          bool             `json:"taskDebug,omitempty"`
	TaskRun            []string         `json:"taskRun,omitempty"`
	TaskRunAutoRestart bool             `json:"taskRunAutoRestart,omitempty"`
	TaskHistory        map[string]uint8 `json:"taskHistory,omitempty"`
}

type Package struct {
	Assembler string `json:"a"`
	Task      string `json:"t"`
	Payload   []byte `json:"p"`
}

type ZeromqServer struct {
	PublicKey string `json:"publicKey"`
	PubPort   uint16 `json:"pubPort"`
	SubPort   uint16 `json:"subPort"`
}

type ServerInfo struct {
	ZeroMQ ZeromqServer `json:"zeromq"`
}
