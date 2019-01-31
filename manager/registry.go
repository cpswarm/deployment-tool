package main

import (
	"fmt"
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
	Build        *build        `json:"build"`
	Deploy       *deploy       `json:"deploy"`
	ChildOrder   string        `json:"childOrder,omitempty"`
	// internal
	receivers      []string
	receiverTopics []string
}

type build struct {
	model.Build `yaml:",inline"`
	Host        string `json:"host"`
}

type deploy struct {
	model.Deploy `yaml:",inline"`
	Target       struct {
		IDs  []string `json:"ids"`
		Tags []string `json:"tags"`
	} `json:"targets"`
}

func (o order) validate() error {
	// validate build
	if o.Build != nil {
		if len(o.Build.Commands) == 0 {
			return fmt.Errorf("no commands for build")
		}
		for _, path := range o.Build.Artifacts {
			if strings.HasPrefix(path, "/") {
				return fmt.Errorf("path to artifact should be relative to source. Given path is absolute: %s", path)
			}
		}
	}

	// validate deploy
	if o.Deploy != nil {
		if len(o.Deploy.Install.Commands)+len(o.Deploy.Run.Commands) == 0 {
			return fmt.Errorf("no install or run commands for deploy")
		}
	}

	return nil
}

// getChild returns the deploy part of order
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
	Stages  map[string]*stage  `json:"stages"`
	Updated model.UnixTimeType `json:"updated"`
}

type stage map[string][]stageLog

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
		t.Logs[id].Stages = make(map[string]*stage)
	}
}

func (logs *logs) insert(l model.Log) {
	if l.Command == "" {
		l.Command = systemLogsKey
	}

	var s stage
	t, ok := logs.Stages[l.Stage]
	if !ok {
		s = make(stage)
	} else {
		s = *t
	}

	commit := func() {
		logs.Stages[l.Stage] = &s
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
