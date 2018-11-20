package main

import (
	"fmt"
	"log"

	"code.linksmart.eu/dt/deployment-tool/model"
)

const (
	SystemLogsKey = "SYS"
)

type registry struct {
	taskDescriptions []TaskDescription
	Targets          map[string]*Target
}

//
// TASK DESCRIPTION
//
type TaskDescription struct {
	Stages Stages
	Target DeploymentTarget
	Debug  bool

	DeploymentInfo DeploymentInfo
}

func (d TaskDescription) validate() error {
	if len(d.Stages.Assemble)+len(d.Stages.Transfer)+len(d.Stages.Install)+len(d.Stages.Test)+len(d.Stages.Run) == 0 {
		return fmt.Errorf("empty stages")
	}
	return nil
}

type Stages struct {
	Assemble []string
	Transfer []string
	Install  []string
	Test     []string
	Run      []string
}

type DeploymentTarget struct {
	Tags []string
}

type DeploymentInfo struct {
	TaskID          string
	Created         string
	TransferSize    int
	MatchingTargets []string
}

//
// TARGET
//
type Target struct {
	Tags           []string
	Tasks          map[string]*Task
	LastLogRequest model.UnixTimeType
}

func newTarget() *Target {
	return &Target{
		Tasks: make(map[string]*Task),
	}
}

func (t *Target) initTask(id string) {
	if _, found := t.Tasks[id]; !found {
		t.Tasks[id] = new(Task)
	}
}

type Task struct {
	Stages  StageLogs
	Updated model.UnixTimeType
}

func (t *Task) GetStageLog(stage string) *StageLog {
	switch stage {
	case model.StageAssemble:
		return &t.Stages.Assemble
	case model.StageTransfer:
		return &t.Stages.Transfer
	case model.StageInstall:
		return &t.Stages.Install
	case model.StageTest:
		return &t.Stages.Test
	case model.StageRun:
		return &t.Stages.Run
	}
	log.Println("ERROR: Unknown/unsupported stage:", stage)
	return &StageLog{}
}

type StageLogs struct {
	Assemble StageLog
	Transfer StageLog
	Install  StageLog
	Test     StageLog
	Run      StageLog
}

type StageLog struct {
	Logs map[string][]Log `json:",omitempty"'`
}

type Log struct {
	Output string
	Error  bool `json:",omitempty"'`
	Time   model.UnixTimeType
}

func (s *StageLog) InsertLogs(l model.Log) {
	if l.Command == "" {
		l.Command = SystemLogsKey
	}
	if s.Logs == nil {
		s.Logs = make(map[string][]Log)
	}

	// TODO make this efficient
	// discard if duplicate
	for _, log := range s.Logs[l.Command] {
		if log.Time == l.Time && log.Output == l.Output {
			return
		}
	}
	// insert into "ordered" logs
	temp := make([]Log, len(s.Logs[l.Command])+1)
	i := 0
	for ti := range temp {
		if len(s.Logs[l.Command]) > i && s.Logs[l.Command][i].Time < l.Time {
			temp[ti] = s.Logs[l.Command][i] // assign the old log
			i++
		} else {
			temp[ti] = Log{l.Output, l.Error, l.Time} // assign the new log
			copy(temp[ti+1:], s.Logs[l.Command][i:])  // copy the remaining logs to the end
			s.Logs[l.Command] = temp
			break
		}
	}
}

func (s *StageLog) Flush() {
	*s = StageLog{}
}
