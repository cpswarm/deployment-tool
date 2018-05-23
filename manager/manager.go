package main

import (
	"bytes"
	"fmt"
	"log"
	"time"

	"code.linksmart.eu/dt/deployment-tool/model"
	"github.com/davecgh/go-spew/spew"
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

func (m *manager) addTaskDescr(descr TaskDescription) (string, error) {

	var compressedArchive []byte
	var err error
	if len(descr.Stages.Transfer) > 0 {
		compressedArchive, err = m.compressFiles(descr.Stages.Transfer)
		if err != nil {
			return "", fmt.Errorf("error compressing files: %s", err)
		}
	}

	task := model.Task{
		ID:        uuid.NewV1().String(),
		Commands:  descr.Stages.Install,
		Artifacts: compressedArchive,
		Time:      time.Now().Unix(),
	}

	//m.tasks = append(m.tasks, task)
	descr.DeploymentInfo.TaskID = task.ID
	descr.DeploymentInfo.TransferSize = len(compressedArchive)
	m.taskDescriptions = append(m.taskDescriptions, descr)

	go m.sendTask(task)

	return task.ID, nil
}

func (m *manager) compressFiles(filePaths []string) ([]byte, error) {
	var b bytes.Buffer
	err := archiver.TarGz.Write(&b, filePaths)
	if err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func (m *manager) sendTask(task model.Task) {
	pending := true

	for pending {
		//log.Printf("sendTasks: %+v", task)
		m.pipe.TaskCh <- task

		time.Sleep(3 * time.Second)

		pending = false
		for _, target := range m.targets {
			if target.Tasks.LatestBatchResponse.TaskID != task.ID {
				pending = true
			}
		}
	}
	log.Println("Task received by all targets.")
}

func (m *manager) processResponses() {
	for response := range m.pipe.ResponseCh {
		spew.Dump(response)

		if _, found := m.targets[response.TargetID]; !found {
			log.Println("Response from unknown target:", response.TargetID)
			continue
		}
		log.Printf("processResponses %+v", response)

		targetTasks := &m.targets[response.TargetID].Tasks
		targetTasks.LatestBatchResponse = response

		if len(targetTasks.History) == 0 {
			targetTasks.History = []string{response.TaskID}
		} else if targetTasks.History[len(targetTasks.History)-1] != response.TaskID {
			targetTasks.History = append(targetTasks.History, response.TaskID)
		}
	}
}
