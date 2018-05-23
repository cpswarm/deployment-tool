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
	Assembly     []string
	Transfer     []string
	Installation []string
	Tests        []string
	Activation   []string
}
