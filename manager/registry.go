package main

import (
	"fmt"
	"log"
	"strings"

	"code.linksmart.eu/dt/deployment-tool/manager/model"
	"code.linksmart.eu/dt/deployment-tool/manager/source"
)

const (
	systemLogsKey = "SYS"
)

type registry struct {
	orders  map[string]*order
	targets map[string]*target
}

//
// ORDER
//
type order struct {
	model.Header `yaml:",inline"`
	Source       source.Source `json:"source"`
	Build        build         `json:"build"`
	Deploy       deploy        `json:"deploy"`
	//Stages       model.Stages  `json:"stages"`
	ChildOrder string `json:"childOrder,omitempty"`
	// internal
	receivers      []string
	receiverTopics []string
}

type build struct {
	Commands  []string `json:"commands"`
	Host      string   `json:"host"`
	Artifacts []string `json:"artifacts"`
}

type deploy struct {
	Install struct {
		Commands []string `json:"commands"`
	} `json:"install"`
	Run struct {
		Commands    []string `json:"commands"`
		AutoRestart bool     `json:"autoRestart"`
	} `json:"run"`
	Target struct {
		Assembler string   `json:"assembler,omitempty"`
		IDs       []string `json:"ids"`
		Tags      []string `json:"tags"`
	} `json:"targets"`
}

func (o order) validate() error {
	if len(o.Build.Commands)+len(o.Deploy.Install.Commands)+len(o.Deploy.Run.Commands) == 0 {
		return fmt.Errorf("empty stages")
	}
	for _, path := range o.Build.Artifacts {
		if strings.HasPrefix(path, "/") {
			return fmt.Errorf("path to artifact should be relative to source. Given path is absolute: %s", path)
		}
	}
	return nil
}

func (o order) getChild() *order {
	var child order
	source := source.Order(o.ID)
	child.Source.Order = &source
	child.Deploy = o.Deploy
	child.Debug = o.Debug
	return &child
}

//
// TARGET
//
type target struct {
	Tags           []string           `json:"tags"`
	Logs           map[string]*logs   `json:"logs"`
	LastLogRequest model.UnixTimeType `json:"lastLogRequest"`
}

type logs struct {
	stages
	Updated model.UnixTimeType `json:"updated"`
}

type stages struct {
	Transfer map[string][]stageLog `json:"transfer"`
	Install  map[string][]stageLog `json:"install"`
	Run      map[string][]stageLog `json:"run"`
}

type stageLog struct {
	Output string             `json:"output"`
	Error  bool               `json:"error,omitempty"`
	Time   model.UnixTimeType `json:"time"`
}

func newTarget() *target {
	return &target{
		Logs: make(map[string]*logs),
	}
}

func (t *target) initTask(id string) {
	if _, found := t.Logs[id]; !found {
		t.Logs[id] = new(logs)
	}
}

func (logs *logs) getStage(stage string) map[string][]stageLog {
	switch stage {
	case model.StageTransfer:
		return logs.stages.Transfer
	case model.StageInstall:
		return logs.stages.Install
	case model.StageRun:
		return logs.stages.Run
	}
	log.Println("ERROR: Unknown/unsupported stage:", stage)
	return nil
}

func (logs *logs) insert(l model.Log) {
	if l.Command == "" {
		l.Command = systemLogsKey
	}

	// TODO this is as ugly as code can get
	s := logs.getStage(l.Stage)
	if s == nil {
		s = make(map[string][]stageLog)
	}
	commit := func() {
		switch l.Stage {
		case model.StageTransfer:
			logs.stages.Transfer = s
		case model.StageInstall:
			logs.stages.Install = s
		case model.StageRun:
			logs.stages.Run = s
		}
		logs.Updated = model.UnixTime()
	}

	// first insertion
	if len(s[l.Command]) == 0 {
		s[l.Command] = append(s[l.Command], stageLog{l.Output, l.Error, l.Time})
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
		s[l.Command] = append(s[l.Command], stageLog{l.Output, l.Error, l.Time})
		commit()
		return
	}
	// insert in the middle
	s[l.Command] = append(s[l.Command], stageLog{})
	copy(s[l.Command][i+1:], s[l.Command][i:])
	s[l.Command][i] = stageLog{l.Output, l.Error, l.Time}
	commit()
}
