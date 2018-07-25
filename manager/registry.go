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

func (s *StageLog) insertLogs(responses []model.Response) {
	s.Logs = append(s.Logs, responses...)
}

func (t *CurrentTask) InsertResponses(b *model.BatchResponse) {
	switch b.Stage {
	case model.StageUnspecified:
		// do nothing
	case model.StageAssemble:
		t.Stages.Assemble.insertLogs(b.Responses)
	case model.StageTransfer:
		t.Stages.Transfer.insertLogs(b.Responses)
	case model.StageInstall:
		t.Stages.Install.insertLogs(b.Responses)
	case model.StageTest:
		t.Stages.Test.insertLogs(b.Responses)
	case model.StageRun:
		t.Stages.Run.insertLogs(b.Responses)
	default:
		log.Fatalln("Unknown/unsupported stage:", b.Stage)
	}
}
