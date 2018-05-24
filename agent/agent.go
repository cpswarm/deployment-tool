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

	log.Println("TargetID", a.ID)
	log.Println("CurrentTask", a.Tasks.LatestBatchResponse.TaskID)
	log.Println("CurrentTaskStatus", a.Tasks.LatestBatchResponse.ResponseType)

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

		// TODO subscribe to next versions
		// For now, drop existing tasks
		for i := len(a.Tasks.History) - 1; i >= 0; i-- {
			if a.Tasks.History[i] == task.ID {
				log.Println("Existing task. Dropping it.")
				continue TASKLOOP
			}
		}
		a.Tasks.History = append(a.Tasks.History, task.ID)

		// send acknowledgement
		a.sendResponse(&model.BatchResponse{ResponseType: model.ResponseACK, TaskID: task.ID, TargetID: a.ID})

		go a.processTask(&task)
	}

}

func (a *agent) processTask(task *model.Task) {
	// set work directory
	wd, _ := os.Getwd()
	wd = fmt.Sprintf("%s/tasks/%s", wd, task.ID)
	log.Println("Task work directory:", wd)

	// decompress and store
	a.storeArtifacts(wd, task.Artifacts)
	// execute and collect results
	a.responseBatchCollector(task, wd, time.Duration(3)*time.Second, a.pipe.ResponseCh)
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
	a.Tasks.LatestBatchResponse = *resp
	a.saveConfig()
}

func (a *agent) close() {
	a.saveConfig()
}
