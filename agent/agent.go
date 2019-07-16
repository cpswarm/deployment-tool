package main

import (
	"bytes"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"code.linksmart.eu/dt/deployment-tool/manager/model"
	"code.linksmart.eu/dt/deployment-tool/manager/source"
	"github.com/pbnjay/memory"
)

const (
	AdvInterval = 60 * time.Second
	TerminalDir = "terminal"
)

type agent struct {
	sync.Mutex

	target *target

	pipe         model.Pipe
	disconnected chan bool
	logger       *logger
	installer    installer
	runner       runner
	terminal     *executor
}

// TODO
// 	make two objects to hold active and pending tasks along with their resources
// 	active task should be persisted for recovery

func startAgent(target *target, managerAddr string) (*agent, error) {

	a := &agent{
		pipe:         model.NewPipe(),
		disconnected: make(chan bool),
	}
	a.target = target

	if !a.target.Registered && os.Getenv(EnvAuthToken) != "" {
		err := a.registerTarget(managerAddr, os.Getenv(EnvAuthToken))
		if err != nil {
			return nil, fmt.Errorf("error registering target: %s", err)
		}
		a.target.Registered = true
		a.target.saveState()
	} else if !a.target.Registered && os.Getenv(EnvAuthToken) == "" {
		return nil, fmt.Errorf("target not registered. Provide token for registration")
	}

	if a.target.ZeromqServerConf.PublicKey == "" || a.target.ZeromqServerConf.PubPort == "" || a.target.ZeromqServerConf.SubPort == "" {
		zmqConf, err := a.getServerInfo(managerAddr)
		if err != nil {
			return nil, fmt.Errorf("error getting server info: %s", err)
		}
		a.target.ZeromqServerConf.ZeromqServerInfo = *zmqConf
		a.target.saveState()
	}

	a.logger = newLogger(a.target.ID, a.pipe.ResponseCh)
	a.runner = newRunner(a.logger.enqueue)
	a.installer = newInstaller(a.logger.enqueue)

	err := a.setupTerminal()
	if err != nil {
		return nil, fmt.Errorf("error setting up terminal: %s", err)
	}

	// autostart
	// TODO check autostart settings
	if len(a.target.TaskRun) > 0 {
		go a.runner.run(a.target.TaskRun, a.target.TaskID, a.target.TaskDebug)
	}

	go a.startWorker()
	return a, nil
}

func (a *agent) setupTerminal() error {
	err := os.MkdirAll(fmt.Sprintf("%s/%s", WorkDir, TerminalDir), 0755)
	if err != nil {
		return fmt.Errorf("error creating terminal directory: %s", err)
	}
	a.terminal = newExecutor(model.TaskTerminal, "", a.logger.priorityEnqueue, true)
	return nil
}

func (a *agent) registerTarget(addr, token string) error {
	log.Println("Registering target...")
	b, err := json.Marshal(a.target.TargetBase)
	if err != nil {
		return fmt.Errorf("error marshalling: %s", err)
	}

	req, err := http.NewRequest(http.MethodPost, addr+"/rpc/targets", bytes.NewBuffer(b))
	if err != nil {
		return fmt.Errorf("error creating request: %s", err)
	}
	req.Header.Set("X-Auth-Token", token)

	client := http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error making request: %s", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("error registering target: %s", resp.Status)
	}
	return nil
}

func (a *agent) getServerInfo(addr string) (*model.ZeromqServerInfo, error) {
	resp, err := http.Get(addr + "/rpc/server_info")
	if err != nil {
		return nil, fmt.Errorf("error making request: %s", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error getting server info: %s", resp.Status)
	}

	decoder := json.NewDecoder(resp.Body)
	var info model.ServerInfo
	err = decoder.Decode(&info)
	if err != nil {
		return nil, fmt.Errorf("error decoding response: %s", err)
	}

	return &info.ZeroMQ, nil
}

func (a *agent) startWorker() {
	topics := a.subscribe()

	log.Println("worker: Waiting for connection and requests...")
	var latestMessageChecksum [16]byte
	for request := range a.pipe.RequestCh {
		log.Println("worker: Request topic:", request.Topic)
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
			} else {
				log.Println("worker: Discarded redundant request")
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
		a.pipe.OperationCh <- model.Operation{model.OperationSubscribe, topic}
	}
	return topics
}

func (a *agent) connected() {
	log.Printf("Connected.")
	defer log.Println("Disconnected!")

	// send first adv after a second. Cancel if disconnected
	//first := time.AfterFunc(time.Second, a.sendAdvertisement)

	//t := time.NewTicker(AdvInterval)
	for {
		select {
		//case <-t.C:
		//	a.sendAdvertisement()
		case <-a.disconnected:
			//t.Stop()
			//first.Stop()
			return
		}
	}
}

func (a *agent) sendAdvertisement() {
	t := model.TargetBase{
		ID:        a.target.ID,
		Tags:      a.target.Tags,
		Location:  a.target.Location,
		PublicKey: a.target.PublicKey,
	}
	log.Println("Sent adv:", t.ID, t.Tags, t.Location)
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
		a.executeCommand(w.Command)
	case w.StopAll != nil:
		a.stopAll()
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

	var stage string
	if taskA.Type == model.TaskTypeBuild {
		stage = model.StageBuild
	} else {
		stage = model.StageInstall
	}

	log.Printf("Received announcement %s/%d", taskA.ID, taskA.Type)
	a.target.TaskHistory[taskA.ID] = taskA.Type
	a.target.saveState()

	//a.sendLog(taskA.ID, stage, model.StageStart, false, taskA.Debug)
	a.sendLog(taskA.ID, stage, "received announcement", false, taskA.Debug)

	if a.assessAnnouncement(taskA) {
		a.pipe.OperationCh <- model.Operation{model.OperationSubscribe, taskA.ID}
		a.sendLog(taskA.ID, stage, "subscribed to task", false, taskA.Debug)
	} else {
		log.Printf("Task is too large to process: %v", taskA.Size)
		a.sendLogFatal(taskA.ID, stage, "not enough memory")
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

	var stage string
	if task.Build != nil {
		stage = model.StageBuild
	} else {
		stage = model.StageInstall
	}

	log.Printf("Received task: %s", task.ID)

	a.pipe.OperationCh <- model.Operation{model.OperationUnsubscribe, task.ID}
	a.sendLog(task.ID, stage, "received task", false, true)

	err = a.saveArtifacts(task.Artifacts, task.ID, stage, task.Debug)
	if err != nil {
		a.sendLogFatal(task.ID, stage, err.Error())
		return
	}

	if task.Build != nil {
		a.build(task.Build, task.ID, task.Debug)
		return
	}
	//a.sendLog(task.ID, model.StageEnd, false, task.Debug)

	success := a.installer.install(task.Deploy.Install.Commands, model.StageInstall, task.ID, task.Debug)
	if success {
		a.runner.stop()             // stop runner for old task
		a.removeOtherTasks(task.ID) // remove old task files
		a.target.TaskRun = task.Deploy.Run.Commands
		a.target.TaskRunAutoRestart = task.Deploy.Run.AutoRestart
		a.target.TaskID = task.ID
		a.target.TaskDebug = task.Debug
		a.target.saveState()

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
			a.sendLogFatal(taskID, model.StageBuild, fmt.Sprintf("error compressing package: %s", err))
			return
		}
		a.sendLog(taskID, model.StageBuild, fmt.Sprintf("compressed built package to %d bytes", len(compressed)), false, debug)

		b, err := json.Marshal(model.Package{a.target.ID, taskID, compressed})
		if err != nil {
			a.sendLogFatal(taskID, model.StageBuild, fmt.Sprintf("error serializing package: %s", err))
			return
		}
		a.pipe.ResponseCh <- model.Message{model.ResponsePackage, b}
		a.sendLog(taskID, model.StageBuild, fmt.Sprintf("sent built package"), false, debug)
		//a.sendLog(taskID, model.StageBuild, model.StageEnd, false, debug)
		// TODO add guaranty of delivery
	}
}

func (a *agent) reportLogs(request *model.LogRequest) {
	log.Println("Received log request since", request.IfModifiedSince)
	a.logger.report(request)
}

func (a *agent) executeCommand(command *string) {
	log.Printf("Remote command exec: %s", *command)

	switch *command {
	case model.TerminalStop:
		if a.terminal.cmd == nil {
			a.sendLog(model.TaskTerminal, "", "nothing to stop", false, true)
			return
		}
		a.terminal.stop()
	default:
		if a.terminal.cmd != nil {
			a.sendLog(model.TaskTerminal, "", "unable to execute: terminal is busy", true, true)
			return
		}
		a.terminal.execute(*command)
	}
}

func (a *agent) stopAll() {
	log.Println("Received stop all request")
	a.installer.stop()
	a.runner.stop()
}

func (a *agent) close() {
	a.installer.stop()
	a.runner.stop()
	// takes time until processes log exit signal
	// TODO return executor.stop from execute and log exit signal when e.cmd.Process.Release() returns
	time.Sleep(time.Second)
	a.logger.stop()
	a.target.saveState()
}
