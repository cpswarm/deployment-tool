package main

import (
	"code.linksmart.eu/dt/deployment-tool/model"
)

type registry struct {
	taskDescriptions []TaskDescription
	//tasks            []model.Task
	targets map[string]*model.Target
}

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
	ByName []string
	ByType []string
}

type DeploymentInfo struct {
	TaskID          string
	TransferSize    int
	MatchingTargets []string
}
