package main

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"reflect"
	"strings"
	"sync"
	"time"

	"code.linksmart.eu/dt/deployment-tool/model"
	"github.com/satori/go.uuid"
)

const (
	AdvInterval = 30 * time.Second
)

type agent struct {
	sync.Mutex

	target model.Target

	pipe         model.Pipe
	disconnected chan bool
	logger       Logger
	installer    installer
	runner       runner
}

// TODO
// 	make two objects to hold active and pending tasks along with their resources
// 	active task should be persisted for recovery

type logCollector interface {
	sendResponse(*model.Response)
}

func startAgent() *agent {

	a := &agent{
		pipe:         model.NewPipe(),
		disconnected: make(chan bool),
	}
	a.loadConf()

	a.logger = NewLogger(a.target.ID, a.target.TaskDebug, a.pipe.ResponseCh)
	a.runner = newRunner(a.logger.Writer())
	a.installer = newInstaller(a.logger.Writer())

	// autostart
	if len(a.target.TaskRun) > 0 {
		go a.runner.run(a.target.TaskRun, a.target.TaskID, a.target.TaskDebug)
	}

	go a.startWorker()
	return a
}

func (a *agent) loadState() error {
	if _, err := os.Stat(StateFile); os.IsNotExist(err) {
		return err
	}

	b, err := ioutil.ReadFile(StateFile)
	if err != nil {
		return fmt.Errorf("error reading state file: %s", err)
	}

	err = json.Unmarshal(b, &a.target)
	if err != nil {
		return fmt.Errorf("error parsing state file: %s", err)
	}

	log.Println("Loaded state file:", StateFile)
	return nil
}

func (a *agent) loadConf() {
	err := a.loadState()
	if err != nil {
		log.Printf("Error loading state file: %s. Starting fresh.", StateFile)
	}

	// LOAD AND REPLACE WITH ENV VARIABLES
	var changed bool

	id := os.Getenv("ID")
	if id == "" && a.target.AutoGenID == "" {
		a.target.AutoGenID = uuid.NewV4().String()
		log.Println("Generated target ID:", a.target.AutoGenID)
		a.target.ID = a.target.AutoGenID
		changed = true
	} else if id == "" && a.target.ID != a.target.AutoGenID {
		log.Println("Taking previously generated ID:", a.target.AutoGenID)
		a.target.ID = a.target.AutoGenID
		changed = true
	} else if id != "" && id != a.target.ID {
		log.Println("Taking ID from env var:", id)
		a.target.ID = id
		changed = true
	}

	var tags []string
	tagsString := os.Getenv("TAGS")
	if tagsString != "" {
		tags = strings.Split(tagsString, ",")
		for i := 0; i < len(tags); i++ {
			tags[i] = strings.TrimSpace(tags[i])
		}
	}
	if !reflect.DeepEqual(tags, a.target.Tags) {
		a.target.Tags = tags
		changed = true
	}

	if changed {
		a.saveState()
	}
}

func (a *agent) startWorker() {
	log.Printf("Subscribing to topics...")
	topicMap := make(map[string]bool)
	for _, tag := range a.target.Tags {
		tag = model.TargetTag(tag)
		a.pipe.OperationCh <- model.Message{model.OperationSubscribe, []byte(tag)}
		topicMap[tag] = true
	}
	a.pipe.OperationCh <- model.Message{model.OperationSubscribe, []byte(model.RequestTargetAll)}
	a.pipe.OperationCh <- model.Message{model.OperationSubscribe, []byte(model.RequestTargetID + model.PrefixSeparator + a.target.ID)}

	log.Println("Waiting for connection and requests...")
	var latestMessageChecksum [16]byte
	for request := range a.pipe.RequestCh {
		switch {
		case request.Topic == model.PipeConnected:
			go a.advertiseTarget()
		case request.Topic == model.PipeDisconnected:
			a.disconnected <- true
		case request.Topic == model.RequestTargetAll:
			// do nothing for now
		case request.Topic == model.TargetTopic(a.target.ID):
			a.sendLogs(request.Payload)
		case topicMap[request.Topic]:
			// an announcement is received as many matching tags but needs to be processed only once
			sum := md5.Sum(request.Payload)
			if latestMessageChecksum != sum {
				go a.handleAnnouncement(request.Payload)
				latestMessageChecksum = sum
			}
		default:
			go a.handleTask(request.Topic, request.Payload)
		}
	}
}

func (a *agent) advertiseTarget() {
	log.Printf("Will advertise target every %s", AdvInterval)
	defer log.Println("Stopped advertisement routine.")

	a.sendAdvertisement()

	t := time.NewTicker(AdvInterval)
	for {
		select {
		case <-t.C:
			a.sendAdvertisement()
		case <-a.disconnected:
			return
		}
	}
}

func (a *agent) handleAnnouncement(payload []byte) {
	var taskA model.TaskAnnouncement
	err := json.Unmarshal(payload, &taskA)
	if err != nil {
		log.Printf("Error parsing announcement: %s", err) // TODO send to manager
	}
	payload = nil // to release memory

	log.Printf("Received announcement: %s", taskA.ID)
	a.sendTransferResponse(taskA.ID, model.StageStart, false, taskA.Debug)

	for i := len(a.target.TaskHistory) - 1; i >= 0; i-- {
		if a.target.TaskHistory[i] == taskA.ID {
			log.Println("Dropping announcement for task", taskA.ID)
			return
		}
	}
	a.pipe.OperationCh <- model.Message{model.OperationSubscribe, []byte(taskA.ID)}

	if a.installer.evaluate(taskA) {
		a.pipe.OperationCh <- model.Message{model.OperationSubscribe, []byte(taskA.ID)}
		a.sendTransferResponse(taskA.ID, "subscribed to task", false, taskA.Debug)
	} else {
		log.Printf("Task is too large to process: %v", taskA.Size)
		a.sendTransferResponse(taskA.ID, "not enough memory", true, taskA.Debug)
	}
}

// TODO make this sequenctial
func (a *agent) handleTask(id string, payload []byte) {
	log.Printf("Received task: %s", id)

	var task model.Task
	err := json.Unmarshal(payload, &task)
	if err != nil {
		log.Printf("Error parsing task: %s", err) // TODO send to manager
	}
	payload = nil // to release memory
	//runtime.GC() ?

	a.pipe.OperationCh <- model.Message{model.OperationUnsubscribe, []byte(task.ID)}
	a.sendTransferResponse(task.ID, "received task", false, task.Debug)
	a.target.TaskHistory = append(a.target.TaskHistory, task.ID)

	a.installer.store(task.Artifacts, task.ID)
	a.sendTransferResponse(task.ID, model.StageEnd, false, task.Debug)

	success := a.installer.install(task.Install, task.ID, task.Debug)
	if success {
		a.runner.stop()            // stop runner for old task
		a.installer.clean(task.ID) // remove old task files
		a.target.TaskRun = task.Run
		a.target.TaskID = task.ID
		a.target.TaskDebug = task.Debug
		a.saveState()

		go a.runner.run(task.Run, task.ID, task.Debug)
	}
}

func (a *agent) sendLogs(payload []byte) {
	var request model.LogRequest
	err := json.Unmarshal(payload, &request)
	if err != nil {
		log.Println("Error parsing log request: %s", err) // TODO send to manager
	}

	a.logger.Report(request)
}

func (a *agent) saveState() {
	a.Lock()
	defer a.Unlock()

	b, _ := json.MarshalIndent(&a.target, "", "\t")
	err := ioutil.WriteFile(StateFile, b, 0600)
	if err != nil {
		log.Printf("Error saving state: %s", err)
		return
	}
	log.Println("Saved state:", StateFile)
}

func (a *agent) sendTransferResponse(taskID, message string, error, debug bool) {
	a.logger.Writer() <- model.Log{taskID, model.StageTransfer, "", message, error, model.UnixTime(), debug}
}

func (a *agent) sendAdvertisement() {
	t := model.Target{
		ID:     a.target.ID,
		Tags:   a.target.Tags,
		TaskID: a.target.TaskID,
	}
	b, _ := json.Marshal(t)
	a.pipe.ResponseCh <- model.Message{string(model.ResponseAdvertisement), b}
}

func (a *agent) close() {
	a.runner.stop()
	a.saveState()
}
