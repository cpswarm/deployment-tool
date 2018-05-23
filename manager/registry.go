package main

import (
	"code.linksmart.eu/dt/deployment-tool/model"
)

type registry struct {
	tasks   []model.Task
	targets map[string]*model.Target
}

type TaskDescription struct {
	Stages Stages
}

type Stages struct {
	Assemble []string
	Transfer []string
	Install  []string
	Test     []string
	Activate []string
}
