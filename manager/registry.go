package main

import (
	"log"

	"code.linksmart.eu/dt/deployment-tool/model"
)

type registry struct {
	taskDescriptions []TaskDescription
	//tasks            []model.Task
	//targets map[string]*model.Target
	Targets map[string]*Target
}

//
// TASK DESCRIPTION
//
type TaskDescription struct {
	Stages Stages
	Target DeploymentTarget
	Log    model.Log

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
	Tags    []string
	Task    Task              // active task
	History map[string]string // task history -> taskID: stage_status
}

type Task struct {
	ID           string
	CurrentStage model.StageType
	Error        bool
	StageLogs    StageLogs
}

func (t *Task) GetStageLog(stage model.StageType) *StageLog {
	switch stage {
	case model.StageUnspecified:
		// do nothing
		return &StageLog{}
	case model.StageAssemble:
		return &t.StageLogs.Assemble
	case model.StageTransfer:
		return &t.StageLogs.Transfer
	case model.StageInstall:
		return &t.StageLogs.Install
	case model.StageTest:
		return &t.StageLogs.Test
	case model.StageRun:
		return &t.StageLogs.Run
	}
	log.Fatalln("Unknown/unsupported stage:", stage)
	return nil
}

type StageLogs struct {
	Assemble StageLog
	Transfer StageLog
	Install  StageLog
	Test     StageLog
	Run      StageLog
}

type StageLog struct {
	Status  model.ResponseType `json:",omitempty"'`
	Updated string             `json:",omitempty"'`
	Logs    []model.Response   `json:",omitempty"'`
}

func (s *StageLog) InsertLogs(responses []model.Response) {
	s.Logs = append(s.Logs, responses...)
}

func (s *StageLog) Flush() {
	*s = StageLog{}
}
