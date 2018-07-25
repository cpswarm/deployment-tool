package main

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"sync"
	"time"

	"code.linksmart.eu/dt/deployment-tool/agent/buffer"
	"code.linksmart.eu/dt/deployment-tool/model"
	"github.com/pbnjay/memory"
	"github.com/satori/go.uuid"
)

const (
	AdvInterval       = 30 * time.Second
	LogBufferCapacity = 10
)

type agent struct {
	sync.Mutex

	target     model.Target
	configPath string
	pipe       model.Pipe
	buf        buffer.Buffer
}

func startAgent() *agent {
	a := &agent{
		pipe:       model.NewPipe(),
		configPath: "config.json",
		buf:        buffer.NewBuffer(LogBufferCapacity),
	}
	a.loadConf()
	if a.target.Tasks == nil {
		a.target.Tasks = new(model.TaskHistory)
	}
	// autostart
	log.Println(a.target.Tasks.Activation)
	if len(a.target.Tasks.Activation) > 0 {
		a.activate(a.target.Tasks.Activation, a.target.Tasks.Logging, a.target.Tasks.LatestBatchResponse.TaskID)
	}

	go a.startWorker()
	return a
}

func (a *agent) loadConf() {
	if _, err := os.Stat(a.configPath); os.IsNotExist(err) {
		log.Println("Configuration file not found.")

		a.target.ID = uuid.NewV4().String()
		log.Println("Generated target ID:", a.target.ID)
		a.saveConfig()
		return
	}

	b, err := ioutil.ReadFile("config.json")
	if err != nil {
		log.Fatal(err)
	}
	err = json.Unmarshal(b, &a.target)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Loaded config file:", a.configPath)

	if a.target.ID == "" {
		a.target.ID = uuid.NewV4().String()
		log.Println("Generated target ID:", a.target.ID)
		a.saveConfig()
	}
}

func (a *agent) startWorker() {
	log.Printf("Subscribing to topics...")
	topicMap := make(map[string]bool)
	for _, topic := range a.target.Tags {
		a.pipe.OperationCh <- model.Message{model.OperationSubscribe, []byte(topic)}
		topicMap[topic] = true
	}
	a.pipe.OperationCh <- model.Message{model.OperationSubscribe, []byte(model.RequestTargetAll)}
	a.pipe.OperationCh <- model.Message{model.OperationSubscribe, []byte(model.RequestTargetID + model.PrefixSeperator + a.target.ID)}

	log.Println("Listenning to requests...")
	var latestMessageChecksum [16]byte
	for request := range a.pipe.RequestCh {
		switch {
		case model.RequestTargetAdvertisement == request.Topic:
			go a.advertiseTarget()
		case model.RequestTargetAll == request.Topic:
			// do nothing for now
		case model.RequestTargetID+model.PrefixSeperator+a.target.ID == request.Topic:
			log.Println(string(request.Payload))
			a.sendRunLogs()
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
	for t := time.NewTicker(AdvInterval); true; <-t.C {
		b, _ := json.Marshal(a.target)
		a.pipe.ResponseCh <- model.Message{model.ResponseAdvertisement, b}
	}
}

func (a *agent) handleAnnouncement(payload []byte) {
	var taskA model.TaskAnnouncement
	err := json.Unmarshal(payload, &taskA)
	if err != nil {
		log.Fatalln(err) // TODO send to manager
	}
	payload = nil // to release memory

	log.Printf("handleAnnouncement: %s", taskA.ID)

	sizeLimit := memory.TotalMemory() / 2 // TODO calculate this based on the available memory
	if taskA.Size <= sizeLimit {
		log.Printf("task announcement. Size: %v", taskA.Size)
		log.Printf("Total system memory: %d\n", memory.TotalMemory())
		for i := len(a.target.Tasks.History) - 1; i >= 0; i-- {
			if a.target.Tasks.History[i] == taskA.ID {
				log.Println("Dropping announcement for task", taskA.ID)
				return
			}
		}
		a.pipe.OperationCh <- model.Message{model.OperationSubscribe, []byte(taskA.ID)}
		a.sendResponse(&model.BatchResponse{ResponseType: model.ResponseAck, TaskID: taskA.ID, TargetID: a.target.ID, Stage: model.StageTransfer})
	} else {
		log.Printf("Task is too large to process: %v", taskA.Size)
		a.sendResponse(&model.BatchResponse{ResponseType: model.ResponseError, TaskID: taskA.ID, TargetID: a.target.ID, Stage: model.StageTransfer}) // TODO include error message
	}

}

func (a *agent) handleTask(id string, payload []byte) {
	log.Printf("handleTask: %s", id)

	var task model.Task
	err := json.Unmarshal(payload, &task)
	if err != nil {
		log.Fatalln(err) // TODO send to manager
	}
	payload = nil // to release memory

	a.pipe.OperationCh <- model.Message{model.OperationUnsubscribe, []byte(task.ID)}
	a.sendResponse(&model.BatchResponse{ResponseType: model.ResponseAckTask, TaskID: task.ID, TargetID: a.target.ID, Stage: model.StageTransfer})
	a.target.Tasks.History = append(a.target.Tasks.History, task.ID)

	// set work directory
	wd, _ := os.Getwd()
	wd = fmt.Sprintf("%s/tasks/%s", wd, task.ID)
	log.Println("Task work directory:", wd)

	// start a new executor
	exec := newExecutor(wd)

	// decompress and store
	exec.storeArtifacts(task.Artifacts)
	task.Artifacts = nil // release memory
	a.sendResponse(&model.BatchResponse{ResponseType: model.ResponseAckTransfer, TaskID: task.ID, TargetID: a.target.ID, Stage: model.StageTransfer})

	// execute and collect results
	resCh := make(chan model.BatchResponse)
	go func() {
		for res := range resCh {
			res.TaskID = task.ID
			res.Stage = model.StageInstall
			a.sendResponse(&res)
		}
	}()
	success := exec.responseBatchCollector(task.Commands, task.Log, resCh)
	if success {
		a.activate(task.Activation, task.Log, task.ID)
	}
}

func (a *agent) sendRunLogs() {
	log.Printf("Sending runner logs to manager.")
	a.sendResponse(&model.BatchResponse{
		ResponseType: model.ResponseRunnerLog,
		Responses:    a.buf.Collect(),
		TargetID:     a.target.ID,
		TaskID:       a.target.Tasks.LatestBatchResponse.TaskID,
		Stage:        model.StageRun,
	})

}

func (a *agent) activate(commands []string, logging model.Log, taskID string) {
	if len(commands) == 0 {
		return
	}
	a.target.Tasks.Activation = commands
	a.target.Tasks.Logging = logging
	a.saveConfig()

	log.Printf("Activating task :%s", taskID)

	wd, _ := os.Getwd()
	wd = fmt.Sprintf("%s/tasks/%s", wd, taskID)
	// start a new executor
	exec := newExecutor(wd)

	// execute and collect results
	resCh := make(chan model.Response)
	go func() {
		for res := range resCh {
			a.buf.Insert(res)
			log.Printf("Activation log: %v", a.buf.Collect())
		}
		log.Printf("Process ended: %s", taskID)
	}()
	go exec.responseCollector(commands, resCh)

}

func (a *agent) saveConfig() {
	a.Lock()
	defer a.Unlock()

	b, err := json.MarshalIndent(&a.target, "", "\t")
	if err != nil {
		log.Println(err)
		return
	}
	err = ioutil.WriteFile(a.configPath, b, 0600)
	if err != nil {
		log.Println("ERROR:", err)
		return
	}
	log.Println("Saved configuration:", a.configPath)
}

func (a *agent) sendResponse(resp *model.BatchResponse) {
	resp.TargetID = a.target.ID
	// serialize
	b, err := json.Marshal(resp)
	if err != nil {
		log.Println(err)
	}
	// send to channel
	a.pipe.ResponseCh <- model.Message{string(resp.ResponseType), b}
	// update the status
	a.target.Tasks.LatestBatchResponse = *resp
	a.saveConfig()
}

func (a *agent) close() {
	a.saveConfig()
}
