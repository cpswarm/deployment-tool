package main

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"code.linksmart.eu/dt/deployment-tool/manager/model"
	"code.linksmart.eu/dt/deployment-tool/manager/source"
	"github.com/pbnjay/memory"
)

const (
	AdvInterval = 60 * time.Second
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

func startAgent() *agent {

	a := &agent{
		pipe:         model.NewPipe(),
		disconnected: make(chan bool),
	}
	a.target.TaskHistory = make(map[string]uint8)
	a.loadConf()

	a.logger = NewLogger(a.target.ID, a.target.TaskDebug, a.pipe.ResponseCh)
	a.runner = newRunner(a.logger)
	a.installer = newInstaller(a.logger)

	// autostart
	if len(a.target.TaskRun) > 0 {
		go a.runner.run(a.target.TaskRun, a.target.TaskID, a.target.TaskDebug)
	}

	go a.startWorker()
	return a
}

func (a *agent) startWorker() {
	topics := a.subscribe()

	log.Println("Waiting for connection and requests...")
	var latestMessageChecksum [16]byte
	for request := range a.pipe.RequestCh {
		log.Println("Request topic:", request.Topic)
		switch {
		case request.Topic == model.PipeConnected:
			go a.connected()
		case request.Topic == model.PipeDisconnected:
			a.disconnected <- true
		case topics[request.Topic]:
			// a request may be received on few topics but needs to be processed only once
			sum := md5.Sum(request.Payload)
			if latestMessageChecksum != sum {
				go a.handleRequest(request.Payload)
				latestMessageChecksum = sum
			}
		default:
			// topic is the task id
			go a.handleTask(request.Payload)
		}
	}
}

func (a *agent) subscribe() map[string]bool {
	log.Printf("Subscribing to topics...")

	topics := make(map[string]bool)
	topics[model.RequestTargetAll] = true
	topics[model.FormatTopicID(a.target.ID)] = true
	for _, tag := range a.target.Tags {
		topics[model.FormatTopicTag(tag)] = true
	}

	for topic := range topics {
		a.pipe.OperationCh <- model.Message{model.OperationSubscribe, []byte(topic)}
	}
	return topics
}

func (a *agent) connected() {
	log.Printf("Connected.")
	defer log.Println("Disconnected!")

	// send first adv after a second. Cancel if disconnected
	first := time.AfterFunc(time.Second, a.sendAdvertisement)

	t := time.NewTicker(AdvInterval)
	for {
		select {
		case <-t.C:
			a.sendAdvertisement()
		case <-a.disconnected:
			t.Stop()
			first.Stop()
			return
		}
	}
}

func (a *agent) sendAdvertisement() {
	t := model.TargetBase{
		ID:   a.target.ID,
		Tags: a.target.Tags,
	}
	log.Println("Sent adv:", t.ID, t.Tags)
	b, _ := json.Marshal(t)
	a.pipe.ResponseCh <- model.Message{model.ResponseAdvertisement, b}
}

func (a *agent) handleRequest(payload []byte) {
	var w model.RequestWrapper
	err := json.Unmarshal(payload, &w)
	if err != nil {
		log.Printf("Error parsing request: %s", err) // TODO send to manager
		return
	}
	payload = nil // to release memory

	// Request is one of these types:
	switch {
	case w.Announcement != nil:
		a.handleAnnouncement(w.Announcement)
	case w.LogRequest != nil:
		a.reportLogs(w.LogRequest)
	case w.Command != nil:
		// TODO execute a single command and send the logs
	default:
		log.Printf("Invalid request: %s->%v", string(payload), w) // TODO send to manager
	}
}

func (a *agent) handleAnnouncement(taskA *model.Announcement) {

	if a.target.TaskHistory[taskA.ID] >= taskA.Type {
		// repeated because other agents expects it or manager hasn't received all acknowledgements
		log.Printf("Dropped repeated announcement %s/%d", taskA.ID, taskA.Type)
		return
	}

	log.Printf("Received announcement %s/%d", taskA.ID, taskA.Type)
	a.target.TaskHistory[taskA.ID] = taskA.Type
	a.saveState()

	a.sendLog(taskA.ID, model.StageStart, false, taskA.Debug)
	a.sendLog(taskA.ID, "received announcement", false, taskA.Debug)

	if a.assessAnnouncement(taskA) {
		a.pipe.OperationCh <- model.Message{model.OperationSubscribe, []byte(taskA.ID)}
		a.sendLog(taskA.ID, "subscribed to task", false, taskA.Debug)
	} else {
		log.Printf("Task is too large to process: %v", taskA.Size)
		a.sendLogFatal(taskA.ID, "not enough memory")
		return
	}
}

func (*agent) assessAnnouncement(ann *model.Announcement) bool {
	sizeLimit := memory.TotalMemory() / 2 // TODO calculate this based on the available memory
	return uint64(ann.Size) <= sizeLimit
}

// TODO make this sequenctial
func (a *agent) handleTask(payload []byte) {
	var task model.Task
	err := json.Unmarshal(payload, &task)
	if err != nil {
		log.Printf("Error parsing task: %s", err) // TODO send to manager
	}
	payload = nil // to release memory
	//runtime.GC() ?

	log.Printf("Received task: %s", task.ID)

	a.pipe.OperationCh <- model.Message{model.OperationUnsubscribe, []byte(task.ID)}
	a.sendLog(task.ID, "received task and unsubscribed", false, task.Debug)

	err = a.saveArtifacts(task.Artifacts, task.ID, task.Debug)
	if err != nil {
		a.sendLogFatal(task.ID, err.Error())
		return
	}

	if task.Build != nil {
		a.build(task.Build, task.ID, task.Debug)
		return
	}
	a.sendLog(task.ID, model.StageEnd, false, task.Debug)

	success := a.installer.install(task.Deploy.Install.Commands, model.StageInstall, task.ID, task.Debug)
	if success {
		a.runner.stop()             // stop runner for old task
		a.removeOtherTasks(task.ID) // remove old task files
		a.target.TaskRun = task.Deploy.Run.Commands
		a.target.TaskRunAutoRestart = task.Deploy.Run.AutoRestart
		a.target.TaskID = task.ID
		a.target.TaskDebug = task.Debug
		a.saveState()

		go a.runner.run(task.Deploy.Run.Commands, task.ID, task.Debug)
	}
}

func (a *agent) build(build *model.Build, taskID string, debug bool) {

	success := a.installer.install(build.Commands, model.StageBuild, taskID, debug)
	if success {
		a.removeOtherTasks(taskID) // remove old task files

		wd := fmt.Sprintf("%s/tasks/%s/%s", WorkDir, taskID, source.SourceDir)
		// make it relative to work directory
		paths := make([]string, len(build.Artifacts))
		for i := range build.Artifacts {
			paths[i] = fmt.Sprintf("%s/%s", wd, build.Artifacts[i])
		}
		compressed, err := model.CompressFiles(paths...)
		if err != nil {
			a.sendLogFatal(taskID, fmt.Sprintf("error compressing package: %s", err))
			return
		}
		a.sendLog(taskID, fmt.Sprintf("compressed built package to %d bytes", len(compressed)), false, debug)

		b, err := json.Marshal(model.Package{a.target.ID, taskID, compressed})
		if err != nil {
			a.sendLogFatal(taskID, fmt.Sprintf("error serializing package: %s", err))
			return
		}
		a.pipe.ResponseCh <- model.Message{model.ResponsePackage, b}
		a.sendLog(taskID, fmt.Sprintf("sent built package"), false, debug)
		a.sendLog(taskID, model.StageEnd, false, debug)
		// TODO add guaranty of delivery
	}
}

func (a *agent) reportLogs(request *model.LogRequest) {
	log.Println("Received log request since", request.IfModifiedSince)
	a.logger.Report(request)
}

func (a *agent) close() {
	a.runner.stop()
	a.saveState()
}
