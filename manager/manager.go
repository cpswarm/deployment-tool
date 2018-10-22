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

	pipe   model.Pipe
	update *sync.Cond
}

func startManager(pipe model.Pipe) (*manager, error) {
	m := &manager{
		pipe:   pipe,
		update: sync.NewCond(&sync.Mutex{}),
	}

	m.Targets = make(map[string]*Target)
	m.taskDescriptions = []TaskDescription{}

	go m.manageResponses()
	return m, nil
}

func (m *manager) addTaskDescr(descr TaskDescription) (*TaskDescription, error) {

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
		Debug:     descr.Debug,
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
	for i := len(split) - 1; i >= 0; i-- {
		reverse = append(reverse, split[i])
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
	// clear status of existing tasks
	for _, target := range m.Targets {
		target.Task = Task{}
	}
	pending := true

	for pending {
		log.Printf("sendTask: %s", task.ID)
		//log.Printf("sendTask: %+v", task)

		// send announcement
		taskA := model.TaskAnnouncement{ID: task.ID, Size: uint64(len(task.Artifacts)), Debug: task.Debug}
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
			if _, found := target.History[task.ID]; !found {
				pending = true
			}
		}
	}
	log.Println("Task received by all targets.")
}

func (m *manager) requestLogs(targetID, stage string) error {
	// TODO send request for missing logs only
	b, _ := json.Marshal(&model.LogRequest{model.UnixTimeType(time.Now().Unix())})
	m.pipe.RequestCh <- model.Message{
		Topic:   model.TargetTopic(targetID),
		Payload: b,
	}
	return nil

}

func (m *manager) manageResponses() {
	for resp := range m.pipe.ResponseCh {
		//log.Printf("startLogManager() %s", resp.Payload)
		switch resp.Topic {
		case string(model.ResponseAdvertisement):
			var target model.Target
			err := json.Unmarshal(resp.Payload, &target)
			if err != nil {
				log.Printf("error parsing response: %s", err)
				log.Printf("payload was: %s", string(resp.Payload))
				continue
			}
			log.Printf("processTarget %+v", target)

			m.Lock()
			if _, found := m.Targets[target.ID]; !found {
				m.Targets[target.ID] = new(Target)
				m.Targets[target.ID].History = make(map[string]string)
			}
			m.Targets[target.ID].Tags = target.Tags
			// create aliases
			task := &m.Targets[target.ID].Task
			stageLogs := task.GetStageLog(target.TaskStage)
			// update current task
			task.ID = target.TaskID
			task.CurrentStage = target.TaskStage
			//task.Error = target.TaskStatus == model.ResponseError // TODO
			//stageLogs.Status = target.TaskStatus // TODO
			stageLogs.Updated = time.Now().Format(time.RFC3339)
			// update history
			m.Targets[target.ID].History[target.TaskID] = m.formatStageStatus(target.TaskStage, "UNKNOWN")
			m.Unlock()
			//log.Println("Received adv", target.ID, target.Tags, target.TaskID, target.TaskStage, target.TaskStatus)
		default:
			var response model.Response
			err := json.Unmarshal(resp.Payload, &response)
			if err != nil {
				log.Printf("error parsing response: %s", err)
				log.Printf("payload was: %s", string(resp.Payload))
				continue
			}

			m.processResponse(&response)
		}
		// sent update notification
		m.update.Broadcast()
	}
}

func (m *manager) processResponse(response *model.Response) {
	log.Printf("processResponse %+v", response)
	//spew.Dump(response)

	if _, found := m.Targets[response.TargetID]; !found {
		log.Println("Log from unknown target:", response.TargetID)
		return
	}

	for _, log := range response.Logs {
		// create aliases
		task := &m.Targets[response.TargetID].Task
		stageLogs := task.GetStageLog(log.Stage)
		// update current task
		task.ID = log.Task
		task.CurrentStage = log.Stage
		stageLogs.InsertLogs(log) // TODO logs not flushed from task to task
		stageLogs.Updated = time.Now().Format(time.RFC3339)
		// update history
		m.Targets[response.TargetID].History[log.Task] = m.formatStageStatus(log.Stage, "UNKNOWN")
	}

}

func (m *manager) formatStageStatus(stage, status string) string {
	return fmt.Sprintf("%s-%s", stage, status)
}
