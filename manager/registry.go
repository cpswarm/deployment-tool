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
	Activate []string
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
	Tasks Tasks
}

type Tasks struct {
	Current CurrentTask
	History map[string]model.ResponseType // id: status
}

type CurrentTask struct {
	ID     string
	Status model.ResponseType
	Stages StageLogs
}

func (t *CurrentTask) Stage(stage model.StageType) *StageLog {
	switch stage {
	case model.StageUnspecified:
		// do nothing
		return nil
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
	RequestedAt string
	Logs        []model.Response
}

func (s *StageLog) InsertLogs(responses []model.Response) {
	s.Logs = append(s.Logs, responses...)
}
