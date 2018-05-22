package main

import (
	"code.linksmart.eu/dt/deployment-tool/model"
)

type registry struct {
	tasks   []model.Task
	targets map[string]*model.Target
}
