package main

import (
	"log"

	"code.linksmart.eu/dt/deployment-tool/model"
)

const (
	SystemLogsKey = "SYS"
)

type registry struct {
	taskDescriptions []TaskDescription
	Targets map[string]*Target
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
	Tags  []string
	Tasks map[string]*Task
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
	Updated string
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
	Error  bool
	Time   model.UnixTimeType
}

func (s *StageLog) InsertLogs(l model.Log) {
	if l.Command == "" {
		l.Command = SystemLogsKey
	}
	if s.Logs == nil {
		s.Logs = make(map[string][]Log)
	}
	s.Logs[l.Command] = append(s.Logs[l.Command], Log{l.Output, l.Error, l.Time})
}

func (s *StageLog) Flush() {
	*s = StageLog{}
}
