package main

import (
	"log"
	"time"

	"code.linksmart.eu/dt/deployment-tool/model"
	"github.com/satori/go.uuid"
)

type manager struct {
	registry
}

func NewManager() (*manager, error) {
	m := &manager{}
	m.targets = make(map[string]*model.Target)

	return m, nil
}

func (m *manager) sendTasks(taskCh chan model.Task) {
	taskID := uuid.NewV4().String()
	pending := true

	for pending {

		//if m.targets["t1"].CurrentTaskStatus >= model.ResponseACK {
		//	log.Println("Task submission is complete.")
		//	os.Exit(0)
		//	break
		//}

		task := model.Task{
			Commands: []string{"pwd"},
			Time:     time.Now().Unix(),
			ID:       taskID,
		}
		log.Printf("sendTasks: %+v", task)
		taskCh <- task

		time.Sleep(3 * time.Second)

		pending = false
		for _, target := range m.targets {
			if target.CurrentTask != taskID {
				pending = true
			}
		}
	}
	log.Println("Task received by all targets.")
}

func (m *manager) processResponses(responseCh chan model.BatchResponse) {
	for response := range responseCh {
		if _, found := m.targets[response.TargetID]; !found {
			log.Println("Response from unknown target:", response.TargetID)
			continue
		}
		log.Printf("processResponses %+v", response)
		m.targets[response.TargetID].CurrentTaskStatus = response.ResponseType
		m.targets[response.TargetID].CurrentTask = response.TaskID

		//spew.Dump(response.TargetID, m.targets[response.TargetID])
	}
}
