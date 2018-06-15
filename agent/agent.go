package main

import (
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

const AdvInterval = 30 * time.Second

type agent struct {
	sync.Mutex
	model.Target
	configPath string

	pipe model.Pipe
}

func startAgent() *agent {
	a := &agent{
		Target:     model.Target{},
		pipe:       model.NewPipe(),
		configPath: "config.json",
	}
	a.loadConf()
	if a.Tasks == nil {
		a.Tasks = new(model.TaskHistory)
	}

	log.Println("TargetID", a.ID)

	go a.startWorker()
	return a
}

func (a *agent) loadConf() {
	if _, err := os.Stat(a.configPath); os.IsNotExist(err) {
		log.Println("Configuration file not found.")
		a.ID = uuid.NewV4().String()
		log.Println("Generated target ID:", a.ID)

		a.saveConfig()
		return
	}

	b, err := ioutil.ReadFile("config.json")
	if err != nil {
		log.Fatal(err)
	}
	err = json.Unmarshal(b, a)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Loaded config file:", a.configPath)

}

func (a *agent) startWorker() {
	log.Println("Listenning to requests...")
	for request := range a.pipe.RequestCh {
		switch request.Topic {
		case model.RequestTargetAdvertisement:
			go a.advertiseTarget()
		case model.RequestTaskAnnouncement:
			go a.handleAnnouncement(request.Payload)
		case a.Target.ID:
			log.Println("Target topic")
			// do nothing
		default:
			go a.handleTask(request.Topic, request.Payload)
		}
	}
}

func (a *agent) advertiseTarget() {
	for t := time.NewTicker(AdvInterval); true; <-t.C {
		a.pipe.ResponseCh <- model.BatchResponse{ResponseType: model.ResponseAdvertisement, TargetID: a.ID}
	}
}

func (a *agent) handleAnnouncement(payload []byte) {
	var taskA model.TaskAnnouncement
	err := json.Unmarshal(payload, &taskA)
	if err != nil {
		log.Fatalln(err) // TODO send to manager
	}
	payload = nil // to release memory

	log.Printf("handleTask: %s", taskA.ID)

	sizeLimit := memory.TotalMemory() / 2 // TODO calculate this based on the available memory
	if taskA.Size <= sizeLimit {
		log.Printf("task announcement. Size: %v", taskA.Size)
		log.Printf("Total system memory: %d\n", memory.TotalMemory())
		for i := len(a.Tasks.History) - 1; i >= 0; i-- {
			if a.Tasks.History[i] == taskA.ID {
				log.Println("Dropping announcement for task", taskA.ID)
				return
			}
		}
		a.sendResponse(&model.BatchResponse{ResponseType: model.ResponseAck, TaskID: taskA.ID, TargetID: a.ID})
	} else {
		log.Printf("Task is too large to process: %v", taskA.Size)
		a.sendResponse(&model.BatchResponse{ResponseType: model.ResponseError, TaskID: taskA.ID, TargetID: a.ID}) // TODO include error message
	}

}

func (a *agent) handleTask(id string, payload []byte) {
	log.Printf("processTask: %s", id)

	var task model.Task
	err := json.Unmarshal(payload, &task)
	if err != nil {
		log.Fatalln(err) // TODO send to manager
	}
	payload = nil // to release memory

	a.sendResponse(&model.BatchResponse{ResponseType: model.ResponseAckTask, TaskID: task.ID, TargetID: a.ID})
	a.Tasks.History = append(a.Tasks.History, task.ID)

	// set work directory
	wd, _ := os.Getwd()
	wd = fmt.Sprintf("%s/tasks/%s", wd, task.ID)
	log.Println("Task work directory:", wd)

	// decompress and store
	a.storeArtifacts(wd, task.Artifacts)
	a.sendResponse(&model.BatchResponse{ResponseType: model.ResponseAckTransfer, TaskID: task.ID, TargetID: a.ID})
	interval, err := time.ParseDuration(task.Log.Interval)
	if err != nil {
		log.Println(err)
		a.sendResponse(&model.BatchResponse{ResponseType: model.ResponseClientError, TaskID: task.ID, TargetID: a.ID})
		return
	}
	log.Println("Will send logs every", interval)

	// execute and collect results
	a.responseBatchCollector(&task, wd, interval, a.pipe.ResponseCh)
}

func (a *agent) saveConfig() {
	a.Lock()
	defer a.Unlock()

	b, err := json.MarshalIndent(a, "", "\t")
	if err != nil {
		log.Println(err)
		return
	}
	err = ioutil.WriteFile(a.configPath, b, 0600)
	if err != nil {
		log.Println("ERROR:", err)
		return
	}
	//log.Println("Saved configuration:", a.configPath)
}

func (a *agent) sendResponse(resp *model.BatchResponse) {
	// send to channel
	a.pipe.ResponseCh <- *resp
	// update the status
	a.Tasks.LatestBatchResponse = *resp
	a.saveConfig()
}

func (a *agent) close() {
	a.saveConfig()
}
