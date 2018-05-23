package main

import (
	"bytes"
	"log"
	"time"

	"code.linksmart.eu/dt/deployment-tool/model"
	"github.com/mholt/archiver"
	"github.com/satori/go.uuid"
)

type manager struct {
	registry

	pipe model.Pipe
}

func newManager(pipe model.Pipe) (*manager, error) {
	m := &manager{
		pipe: pipe,
	}

	m.targets = make(map[string]*model.Target)

	return m, nil
}

func (m *manager) sendTask(descr TaskDescription) {
	taskID := uuid.NewV4().String()
	pending := true

	// compress archive
	var b bytes.Buffer
	err := archiver.TarGz.Write(&b, descr.Stages.Transfer)
	if err != nil {
		log.Fatal(err)
	}

	for pending {

		task := model.Task{
			Commands:  descr.Stages.Install,
			Artifacts: b.Bytes(),
			Time:      time.Now().Unix(),
			ID:        taskID,
		}
		//log.Printf("sendTasks: %+v", task)
		m.pipe.TaskCh <- task

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

func (m *manager) processResponses() {
	for response := range m.pipe.ResponseCh {
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
