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

	"code.linksmart.eu/dt/deployment-tool/model"
	"github.com/pbnjay/memory"
	"github.com/satori/go.uuid"
)

const (
	AdvInterval       = 30 * time.Second
	LogBufferCapacity = 100
)

type agent struct {
	sync.Mutex

	target       model.Target
	configPath   string
	pipe         model.Pipe
	disconnected chan bool
	runner       *runner
}

func startAgent() *agent {

	a := &agent{
		pipe:         model.NewPipe(),
		configPath:   "config.json",
		disconnected: make(chan bool),
		runner:       newRunner(),
	}

	a.loadConf()
	if a.target.Tasks == nil {
		a.target.Tasks = new(model.TaskHistory)
	}
	// autostart
	if len(a.target.Tasks.Run) > 0 {
		go a.runner.run(a.target.Tasks.Run, a.target.Tasks.LatestBatchResponse.TaskID)
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
	for _, tag := range a.target.Tags {
		tag = model.TargetTag(tag)
		a.pipe.OperationCh <- model.Message{model.OperationSubscribe, []byte(tag)}
		topicMap[tag] = true
	}
	a.pipe.OperationCh <- model.Message{model.OperationSubscribe, []byte(model.RequestTargetAll)}
	a.pipe.OperationCh <- model.Message{model.OperationSubscribe, []byte(model.RequestTargetID + model.PrefixSeperator + a.target.ID)}

	log.Println("Listenning to requests...")
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

	b, _ := json.Marshal(a.target)
	a.pipe.ResponseCh <- model.Message{model.ResponseAdvertisement, b}

	t := time.NewTicker(AdvInterval)
	for {
		select {
		case <-t.C:
			b, _ := json.Marshal(a.target)
			a.pipe.ResponseCh <- model.Message{model.ResponseAdvertisement, b}
		case <-a.disconnected:
			return
		}
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
		a.sendTransferResponse(model.ResponseLog, taskA.ID, "received announcement")
	} else {
		log.Printf("Task is too large to process: %v", taskA.Size)
		a.sendTransferResponse(model.ResponseError, taskA.ID, "not enough memory")
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
	//runtime.GC() ?

	a.pipe.OperationCh <- model.Message{model.OperationUnsubscribe, []byte(task.ID)}
	a.sendTransferResponse(model.ResponseLog, task.ID, "received task")
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
	a.sendTransferResponse(model.ResponseSuccess, task.ID, "stored artifacts")

	// execute and collect results
	resCh := make(chan model.BatchResponse)
	go func() {
		for res := range resCh {
			res.TaskID = task.ID
			res.Stage = model.StageInstall
			a.sendResponse(&res)
		}
	}()
	success := exec.executeAndCollectBatch(task.Install, task.Log, resCh)
	if success {
		a.target.Tasks.Run = task.Run
		a.saveConfig()
		go a.runner.run(task.Run, task.ID)
	}
}

func (a *agent) sendLogs(payload []byte) {
	var request model.LogRequest
	err := json.Unmarshal(payload, &request)
	if err != nil {
		log.Fatalln(err) // TODO send to manager
	}

	log.Printf("Sending logs for: %s", request.Stage)
	switch request.Stage {
	case model.StageRun:
		a.sendResponse(&model.BatchResponse{
			ResponseType: model.ResponseLog,
			Responses:    a.runner.buf.Collect(),
			TargetID:     a.target.ID,
			TaskID:       a.target.Tasks.LatestBatchResponse.TaskID,
			Stage:        model.StageRun,
		})
	default:
		log.Printf("Enexpected stage: %s", request.Stage)
	}

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

func (a *agent) sendTransferResponse(status model.ResponseType, taskID, message string) {
	a.sendResponse(&model.BatchResponse{
		Stage:        model.StageTransfer,
		ResponseType: status,
		TaskID:       taskID,
		Responses:    []model.Response{{Output: message, Error: status == model.ResponseError}},
	})
}

func (a *agent) close() {
	a.runner.stop()
	a.saveConfig()
}
