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

	m.Targets = make(map[string]*Target)
	m.taskDescriptions = []TaskDescription{}

	go m.processResponses()
	return m, nil
}

func (m *manager) addTaskDescr(descr TaskDescription) (*TaskDescription, error) {

	if len(descr.Stages.Run) > 1 {
		return nil, fmt.Errorf("activation request error: execution of multiple processes is currently not supported")
	}

	m.RLock()
TARGETS:
	for id, target := range m.Targets {
		for _, t := range target.Tags {
			for _, t2 := range descr.Target.Tags {
				if t == t2 {
					descr.DeploymentInfo.MatchingTargets = append(descr.DeploymentInfo.MatchingTargets, id)
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
		ID:        newTaskID(),
		Artifacts: compressedArchive,
		Install:   descr.Stages.Install,
		Run:       descr.Stages.Run,
		Log:       descr.Log,
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
			m.pipe.RequestCh <- model.Message{model.TargetTag(tag), b}
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
		for _, target := range m.Targets {
			if _, found := target.Tasks.History[task.ID]; !found {
				pending = true
			}
		}
	}
	log.Println("Task received by all targets.")
}

func (m *manager) requestLogs(targetID string, stage model.StageType) error {
	b, _ := json.Marshal(&model.LogRequest{stage})
	m.pipe.RequestCh <- model.Message{
		Topic:   model.TargetTopic(targetID),
		Payload: b,
	}
	m.Targets[targetID].Tasks.Current.GetStageLogs(stage).RequestedAt = time.Now().Format(time.RFC3339)
	return nil

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

			m.Lock()
			if _, found := m.Targets[target.ID]; !found {
				m.Targets[target.ID] = new(Target)
				m.Targets[target.ID].Tasks.History = make(map[string]model.ResponseType)
			}
			m.Targets[target.ID].Tags = target.Tags
			m.Targets[target.ID].Tasks.Current.ID = target.Tasks.LatestBatchResponse.TaskID
			m.Targets[target.ID].Tasks.Current.CurrentStage = target.Tasks.LatestBatchResponse.Stage
			m.Targets[target.ID].Tasks.Current.Status = target.Tasks.LatestBatchResponse.ResponseType
			m.Targets[target.ID].Tasks.History[target.Tasks.LatestBatchResponse.TaskID] = target.Tasks.LatestBatchResponse.ResponseType
			m.Unlock()
			log.Printf("Received target advertisement: %s Tags: %s", target.ID, target.Tags)
		default:
			var response model.BatchResponse
			err := json.Unmarshal(resp.Payload, &response)
			if err != nil {
				log.Printf("error parsing response: %s", err)
				log.Printf("payload was: %s", string(resp.Payload))
				continue
			}

			//spew.Dump(response)

			if _, found := m.Targets[response.TargetID]; !found {
				log.Println("Response from unknown target:", response.TargetID)
				continue
			}
			log.Printf("processResponses %+v", response)

			m.Lock()
			if len(response.Responses) == 0 {
				// TODO temporary fix for payload-less responses
				response.Responses = append(response.Responses, model.Response{Output: string(response.ResponseType)})
			}
			m.Targets[response.TargetID].Tasks.Current.ID = response.TaskID
			m.Targets[response.TargetID].Tasks.Current.CurrentStage = response.Stage
			m.Targets[response.TargetID].Tasks.Current.Status = response.ResponseType
			m.Targets[response.TargetID].Tasks.Current.GetStageLogs(response.Stage).InsertLogs(response.Responses) // TODO logs not flushed from task to task
			m.Targets[response.TargetID].Tasks.History[response.TaskID] = response.ResponseType
			m.Unlock()
		}
	}
}
