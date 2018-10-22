package main

import (
	"log"
	"sync"

	"code.linksmart.eu/dt/deployment-tool/model"
)

type runner struct {
	logger    chan<- model.Log
	executors []*executor
	wg        sync.WaitGroup
	//shutdown  chan bool
}

func newRunner(logger chan<- model.Log) runner {
	return runner{
		logger: logger,
		//shutdown: make(chan bool, 1),
	}
}

func (r *runner) run(commands []string, taskID string) {
	r.executors = make([]*executor, len(commands))

	// nothing to run
	if len(commands) == 0 {
		return
	}

	log.Printf("run() Running task: %s", taskID)
	r.sendLog(taskID, "", model.StageStart, false, model.UnixTime())
	defer r.sendLog(taskID, "", model.StageEnd, false, model.UnixTime())

	// run in parallel and wait for them to finish
	for i, command := range commands {
		r.executors[i] = newExecutor(taskID, model.StageRun, r.logger)
		r.wg.Add(1)
		go func(c string, e *executor) {
			defer r.wg.Done()
			e.execute(c)
		}(command, r.executors[i])
	}
	r.wg.Wait()

	log.Println("run() All processes are ended.")
}

func (r *runner) sendLog(task, command, output string, error bool, time model.UnixTimeType) {
	r.logger <- model.Log{task, model.StageRun, command, output, error, time}
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

	//<-r.shutdown // wait for all logs
	log.Println("stop() Success:", success)
	return success
}
