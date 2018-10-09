package main

import (
	"fmt"
	"log"
	"os"
	"sync"

	"code.linksmart.eu/dt/deployment-tool/model"
)

type runner struct {
	logger LogWriter
	executors []*executor
	wg        sync.WaitGroup
	shutdown  chan bool
}

func newRunner(logger LogWriter) runner {
	return runner{
		logger: logger,
		shutdown: make(chan bool, 1),
	}
}

func (r *runner) run(commands []string, taskID string) {
	r.executors = make([]*executor, len(commands))

	// nothing to run
	if len(commands) == 0 {
		return
	}

	log.Printf("run() Running task: %s", taskID)
	r.sendRunResponse(taskID, &model.Log{Output: model.ProcessStart}) // TODO report successful start of each process

	wd, _ := os.Getwd()
	wd = fmt.Sprintf("%s/tasks/%s", wd, taskID)

	resCh := make(chan model.Log)
	go func() {
		execError := false
		for res := range resCh {
			if res.Error {
				execError = true
			}

			r.sendRunResponse(taskID, &res)
			log.Printf("run() %v", res)
		}

		r.sendRunResponse(taskID, &model.Log{Output: model.ProcessExit, Error: execError})

		r.shutdown <- true
		log.Printf("run() Closing collector routine.")
	}()

	// run in parallel and wait for them to finish
	for i := 0; i < len(commands); i++ {
		r.executors[i] = newExecutor(wd)
		r.wg.Add(1)
		go r.executors[i].executeAndCollectWg([]string{commands[i]}, resCh, &r.wg)
	}
	r.wg.Wait()

	close(resCh)
	log.Println("run() All processes are ended.")
}

// TODO add the command to identify logs on manager
func (r *runner) sendRunResponse(taskID string, logM *model.Log) {
	log.Println("sendRunResponse()", logM)
	r.logger.Insert(model.StageRun, logM)
}

func (r *runner) stop() bool {
	if len(r.executors) == 0 {
		return true
	}
	log.Println("stop() Shutting down the runner...")
	var success bool
	for i := 0; i < len(r.executors); i++ {
		success = r.executors[i].stop()
	}

	<-r.shutdown // wait for all logs
	log.Println("stop() Success:", success)
	return success
}
