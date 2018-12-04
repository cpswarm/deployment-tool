package main

import (
	"fmt"
	"log"

	"code.linksmart.eu/dt/deployment-tool/manager/model"
)

const (
	SystemLogsKey = "SYS"
)

type registry struct {
	orders  map[string]*Order
	targets map[string]*Target
}

//
// Order
//
type Order struct {
	model.Header `yaml:",inline"`
	Stages       model.Stages `json:"stages"`
	Target       struct {
		IDs  []string `json:"ids"`
		Tags []string `json:"tags"`
	} `json:"targets"`
	Receivers []string `json:"receivers"`
}

func (o Order) Validate() error {
	if len(o.Stages.Transfer)+len(o.Stages.Install)+len(o.Stages.Run) == 0 {
		return fmt.Errorf("empty stages")
	}
	return nil
}

//
// TARGET
//
type Target struct {
	Tags           []string           `json:"tags"`
	Logs           map[string]*Logs   `json:"logs"`
	LastLogRequest model.UnixTimeType `json:"lastLogRequest"`
}

func newTarget() *Target {
	return &Target{
		Logs: make(map[string]*Logs),
	}
}

func (t *Target) initTask(id string) {
	if _, found := t.Logs[id]; !found {
		t.Logs[id] = new(Logs)
	}
}

type Logs struct {
	Stages
	Updated model.UnixTimeType `json:"updated"`
}

func (logs *Logs) GetStageLog(stage string) map[string][]Log {
	switch stage {
	case model.StageTransfer:
		return logs.Stages.Transfer
	case model.StageInstall:
		return logs.Stages.Install
	case model.StageRun:
		return logs.Stages.Run
	}
	log.Println("ERROR: Unknown/unsupported stage:", stage)
	return nil
}

type Stages struct {
	Transfer map[string][]Log `json:"transfer"`
	Install  map[string][]Log `json:"install"`
	Run      map[string][]Log `json:"run"`
}

type Log struct {
	Output string             `json:"output"`
	Error  bool               `json:"error,omitempty"`
	Time   model.UnixTimeType `json:"time"`
}

func (logs *Logs) InsertLogs(l model.Log) {
	if l.Command == "" {
		l.Command = SystemLogsKey
	}

	// TODO this is as ugly as code can get
	s := logs.GetStageLog(l.Stage)
	if s == nil {
		s = make(map[string][]Log)
	}
	commit := func() {
		switch l.Stage {
		case model.StageTransfer:
			logs.Stages.Transfer = s
		case model.StageInstall:
			logs.Stages.Install = s
		case model.StageRun:
			logs.Stages.Run = s
		}
		logs.Updated = model.UnixTime()
	}

	// first insertion
	if len(s[l.Command]) == 0 {
		s[l.Command] = append(s[l.Command], Log{l.Output, l.Error, l.Time})
		commit()
		return
	}

	i := 0
	for ; i < len(s[l.Command]); i++ {
		log := s[l.Command][i]
		// discard if duplicate
		if log.Time == l.Time && log.Output == l.Output {
			return
		}
		// find the index where it should be inserted
		if i == len(s[l.Command])-1 || (l.Time >= log.Time && l.Time < s[l.Command][i+1].Time) {
			i++
			break
		}
	}
	// append to the end
	if i == len(s[l.Command]) {
		s[l.Command] = append(s[l.Command], Log{l.Output, l.Error, l.Time})
		commit()
		return
	}
	// InsertLogs in the middle
	s[l.Command] = append(s[l.Command], Log{})
	copy(s[l.Command][i+1:], s[l.Command][i:])
	s[l.Command][i] = Log{l.Output, l.Error, l.Time}
	commit()
}
