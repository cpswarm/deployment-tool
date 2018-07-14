package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"code.linksmart.eu/dt/deployment-tool/model"
	"github.com/davecgh/go-spew/spew"
	"github.com/mholt/archiver"
	"github.com/satori/go.uuid"
)

type manager struct {
	sync.RWMutex
	registry

	pipe model.Pipe
}

func startManager(pipe model.Pipe) (*manager, error) {
	m := &manager{
		pipe: pipe,
	}

	m.targets = make(map[string]*model.Target)
	m.taskDescriptions = []TaskDescription{}

	go m.processResponses()
	return m, nil
}

func (m *manager) addTaskDescr(descr TaskDescription) (*TaskDescription, error) {

	if len(descr.Stages.Activate) > 1 {
		return nil, fmt.Errorf("activation request error: execution of multiple processes is currently not supported")
	}

	m.RLock()
TARGETS:
	for _, target := range m.targets {
		for _, t := range target.Tags {
			for _, t2 := range descr.Target.Tags {
				if t == t2 {
					descr.DeploymentInfo.MatchingTargets = append(descr.DeploymentInfo.MatchingTargets, target.ID)
					continue TARGETS
				}
			}
		}
	}
	m.RUnlock()

	var compressedArchive []byte
	var err error
	if len(descr.Stages.Transfer) > 0 {
		compressedArchive, err = m.compressFiles(descr.Stages.Transfer)
		if err != nil {
			return nil, fmt.Errorf("error compressing files: %s", err)
		}
	}

	task := model.Task{
		ID:         newTaskID(),
		Commands:   descr.Stages.Install,
		Artifacts:  compressedArchive,
		Activation: descr.Stages.Activate,
		Log:        descr.Log,
	}

	//m.tasks = append(m.tasks, task)
	descr.DeploymentInfo.TaskID = task.ID
	descr.DeploymentInfo.Created = time.Now().Format(time.RFC3339)
	descr.DeploymentInfo.TransferSize = len(compressedArchive)
	m.taskDescriptions = append(m.taskDescriptions, descr)

	go m.sendTask(&task, descr.Target.Tags)

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

func (m *manager) sendTask(task *model.Task, targetTags []string) {
	pending := true

	for pending {
		log.Printf("sendTask: %s", task.ID)
		//log.Printf("sendTask: %+v", task)

		// send announcement
		taskA := model.TaskAnnouncement{ID: task.ID, Size: uint64(len(task.Artifacts))}
		b, err := json.Marshal(&taskA)
		if err != nil {
			log.Fatal(err)
		}
		for _, tag := range targetTags {
			m.pipe.RequestCh <- model.Message{tag, b}
		}

		time.Sleep(time.Second)

		// send actual task
		b, err = json.Marshal(&task)
		if err != nil {
			log.Fatal(err)
		}
		m.pipe.RequestCh <- model.Message{task.ID, b}

		time.Sleep(10 * time.Second)

		// TODO which messages are received, what is pending?
		pending = false
		for _, target := range m.targets {
			if target.Tasks == nil || target.Tasks.LatestBatchResponse.TaskID != task.ID {
				pending = true
			}
		}
	}
	log.Println("Task received by all targets.")
}

func (m *manager) processResponses() {
	for resp := range m.pipe.ResponseCh {
		switch resp.Topic {
		case model.ResponseAdvertisement:
			var target model.Target
			err := json.Unmarshal(resp.Payload, &target)
			if err != nil {
				log.Printf("error parsing response: %s", err)
				log.Printf("payload was: %s", string(resp.Payload))
				continue
			}
			m.targets[target.ID] = &target
			log.Printf("Received target advertisement: %s Tags: %s", target.ID, target.Tags)
		default:
			var response model.BatchResponse
			err := json.Unmarshal(resp.Payload, &response)
			if err != nil {
				log.Printf("error parsing response: %s", err)
				log.Printf("payload was: %s", string(resp.Payload))
				continue
			}

			spew.Dump(response)

			if _, found := m.targets[response.TargetID]; !found {
				log.Println("Response from unknown target:", response.TargetID)
				continue
			}
			log.Printf("processResponses %+v", response)

			m.Lock()
			// allocate memory and work on the alias
			if m.targets[response.TargetID].Tasks == nil {
				m.targets[response.TargetID].Tasks = new(model.TaskHistory)
			}
			m.Unlock()
			targetTask := m.targets[response.TargetID].Tasks

			targetTask.LatestBatchResponse = response
			if len(targetTask.History) == 0 {
				targetTask.History = []string{response.TaskID}
			} else if targetTask.History[len(targetTask.History)-1] != response.TaskID {
				targetTask.History = append(targetTask.History, response.TaskID)
			}
		}
	}
}
