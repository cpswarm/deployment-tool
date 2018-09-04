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
	"github.com/pbnjay/memory"
	"github.com/satori/go.uuid"
)

const (
	AdvInterval       = 30 * time.Second
	LogBufferCapacity = 100
)

type agent struct {
	sync.Mutex

	target model.Target

	pipe         model.Pipe
	disconnected chan bool
	runner       *runner
}

func startAgent() *agent {

	a := &agent{
		pipe:         model.NewPipe(),
		disconnected: make(chan bool),
		runner:       newRunner(),
	}

	a.loadConf()

	// autostart
	if len(a.target.TaskRun) > 0 {
		go a.run(a.target.TaskRun, a.target.TaskID)
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
		log.Println("Unable to load state file. Starting fresh.")
	}

	var changed bool

	// LOAD ENV VARIABLES
	id := os.Getenv("ID")
	if id == "" && a.target.AutoGenID == "" {
		a.target.AutoGenID = uuid.NewV4().String()
		log.Println("Generated target ID:", a.target.AutoGenID)
		a.target.ID = a.target.AutoGenID
		changed = true
	} else if id == "" {
		a.target.ID = a.target.AutoGenID
		changed = true
	} else {
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
		log.Fatalln(err) // TODO send to manager
	}
	payload = nil // to release memory

	log.Printf("handleAnnouncement: %s", taskA.ID)

	sizeLimit := memory.TotalMemory() / 2 // TODO calculate this based on the available memory
	if taskA.Size <= sizeLimit {
		log.Printf("task announcement. Size: %v", taskA.Size)
		log.Printf("Total system memory: %d\n", memory.TotalMemory())
		for i := len(a.target.TaskHistory) - 1; i >= 0; i-- {
			if a.target.TaskHistory[i] == taskA.ID {
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
	a.target.TaskHistory = append(a.target.TaskHistory, task.ID)

	// set work directory
	wd, _ := os.Getwd()
	wd = fmt.Sprintf("%s/tasks", wd)
	taskDir := fmt.Sprintf("%s/%s", wd, task.ID)
	log.Println("Task work directory:", taskDir)

	// start a new executor
	exec := newExecutor(taskDir)

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
		a.runner.stop()              // stop runner for old task
		a.removeOldTask(wd, task.ID) // remove old task files
		a.target.TaskRun = task.Run
		a.saveConfig()

		go a.run(task.Run, task.ID)
	}
}

func (a *agent) removeOldTask(wd, taskID string) {
	_, err := os.Stat(wd)
	if err != nil && os.IsNotExist(err) {
		// nothing to remove
		return
	}
	files, err := ioutil.ReadDir(wd)
	if err != nil {
		log.Printf("Error reading work dir: %s", err)
		return
	}
	for i := 0; i < len(files); i++ {
		if files[i].Name() != taskID {
			filename := fmt.Sprintf("%s/%s", wd, files[i].Name())
			log.Printf("Removing old task dir: %s", files[i].Name())
			err = os.RemoveAll(filename)
			if err != nil {
				log.Printf("Error removing old task dir: %s", err)
			}
		}
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
			ResponseType: a.target.TaskStatus,
			Responses:    a.runner.buf.Collect(),
			TargetID:     a.target.ID,
			TaskID:       a.target.TaskID,
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
	err = ioutil.WriteFile(StateFile, b, 0600)
	if err != nil {
		log.Println("ERROR:", err)
		return
	}
	log.Println("Saved configuration:", StateFile)
}

func (a *agent) sendResponse(resp *model.BatchResponse) {
	a.target.TaskID = resp.TaskID
	a.target.TaskStage = resp.Stage
	a.target.TaskStatus = resp.ResponseType
	a.saveConfig()

	resp.TargetID = a.target.ID

	b, _ := json.Marshal(resp)
	a.pipe.ResponseCh <- model.Message{string(resp.ResponseType), b}
}

func (a *agent) sendTransferResponse(status model.ResponseType, taskID, message string) {
	a.sendResponse(&model.BatchResponse{
		Stage:        model.StageTransfer,
		ResponseType: status,
		TaskID:       taskID,
		Responses:    []model.Response{{Output: message, Error: status == model.ResponseError}},
	})
}

func (a *agent) sendAdvertisement() {
	t := model.Target{
		ID:         a.target.ID,
		Tags:       a.target.Tags,
		TaskID:     a.target.TaskID,
		TaskStage:  a.target.TaskStage,
		TaskStatus: a.target.TaskStatus,
	}
	b, _ := json.Marshal(t)
	a.pipe.ResponseCh <- model.Message{string(model.ResponseAdvertisement), b}
}

func (a *agent) close() {
	a.runner.stop()
	a.saveConfig()
}
