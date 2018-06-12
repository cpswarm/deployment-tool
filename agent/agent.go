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

type agent struct {
	sync.Mutex
	model.Target
	configPath string

	pipe model.Pipe
}

func newAgent(pipe model.Pipe) *agent {
	a := &agent{
		Target:     model.Target{},
		pipe:       pipe,
		configPath: "config.json",
	}
	a.loadConf()
	if a.Task == nil {
		a.Task = new(model.TargetTask)
	}

	log.Println("TargetID", a.ID)

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

func (a *agent) startTaskProcessor() {
	log.Println("Listenning for tasks...")

TASKLOOP:
	for task := range a.pipe.TaskCh {
		//log.Printf("taskProcessor: %+v", task)
		log.Printf("taskProcessor: %s", task.ID)

		if task.Announcement {
			sizeLimit := memory.TotalMemory() / 2 // TODO calculate this based on the available memory
			if task.Size <= sizeLimit {
				log.Printf("task announcement. Size: %v", task.Size)
				log.Printf("Total system memory: %d\n", memory.TotalMemory())
				for i := len(a.Task.History) - 1; i >= 0; i-- {
					if a.Task.History[i] == task.ID {
						log.Println("Dropping announcement for task", task.ID)
						continue TASKLOOP
					}
				}
				a.sendResponse(&model.BatchResponse{ResponseType: model.ResponseAck, TaskID: task.ID, TargetID: a.ID})
			} else {
				log.Printf("Task is too large to process: %v", task.Size)
				a.sendResponse(&model.BatchResponse{ResponseType: model.ResponseError, TaskID: task.ID, TargetID: a.ID}) // TODO include error message
			}

		} else {
			// actual task received
			a.sendResponse(&model.BatchResponse{ResponseType: model.ResponseAckTask, TaskID: task.ID, TargetID: a.ID})
			a.Task.History = append(a.Task.History, task.ID)
			go a.processTask(&task)
		}
	}

}

func (a *agent) processTask(task *model.Task) {
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
	a.responseBatchCollector(task, wd, interval, a.pipe.ResponseCh)
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
	log.Println("Saved configuration:", a.configPath)
}

func (a *agent) sendResponse(resp *model.BatchResponse) {
	// send to channel
	a.pipe.ResponseCh <- *resp
	// update the status
	a.Task.LatestBatchResponse = *resp
	a.saveConfig()
}

func (a *agent) close() {
	a.saveConfig()
}
