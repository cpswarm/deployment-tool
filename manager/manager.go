package main

import (
	"bytes"
	"fmt"
	"log"
	"strings"
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
	m.taskDescriptions = []TaskDescription{}

	return m, nil
}

func (m *manager) addTaskDescr(descr TaskDescription) (*TaskDescription, error) {

	var compressedArchive []byte
	var err error
	if len(descr.Stages.Transfer) > 0 {
		compressedArchive, err = m.compressFiles(descr.Stages.Transfer)
		if err != nil {
			return nil, fmt.Errorf("error compressing files: %s", err)
		}
	}

	task := model.Task{
		ID:        newTaskID(),
		Commands:  descr.Stages.Install,
		Artifacts: compressedArchive,
		Time:      time.Now().Unix(),
		Log:       descr.Log,
		Size:      uint64(len(compressedArchive)),
	}

	//m.tasks = append(m.tasks, task)
	descr.DeploymentInfo.TaskID = task.ID
	descr.DeploymentInfo.TransferSize = len(compressedArchive)
	m.taskDescriptions = append(m.taskDescriptions, descr)

	go m.sendTask(task)

	return &descr, nil
}

func newTaskID() string {
	// inverse the UUIDv1 chunks to make them alphanumerically sortable
	split := strings.Split(uuid.NewV1().String(), "-")
	var reverse []string
	for _, chunk := range split {
		reverse = append([]string{chunk}, reverse...)
	}
	return strings.Join(reverse, "-")
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
		log.Printf("sendTask: %s", task.ID)
		//log.Printf("sendTask: %+v", task)

		m.pipe.TaskCh <- model.Task{ID: task.ID, Size: task.Size, Announcement: true}
		time.Sleep(time.Second)
		m.pipe.TaskCh <- task

		time.Sleep(10 * time.Second)

		// TODO which messages are received, what is pending?
		pending = false
		for _, target := range m.targets {
			if target.Task == nil || target.Task.LatestBatchResponse.TaskID != task.ID {
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

		// allocate memory and work on the alias
		if m.targets[response.TargetID].Task == nil {
			m.targets[response.TargetID].Task = new(model.TargetTask)
		}
		targetTask := m.targets[response.TargetID].Task

		targetTask.LatestBatchResponse = response
		if len(targetTask.History) == 0 {
			targetTask.History = []string{response.TaskID}
		} else if targetTask.History[len(targetTask.History)-1] != response.TaskID {
			targetTask.History = append(targetTask.History, response.TaskID)
		}
	}
}
