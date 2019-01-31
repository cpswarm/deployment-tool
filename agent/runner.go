package main

import (
	"log"
	"sync"

	"code.linksmart.eu/dt/deployment-tool/manager/model"
)

type runner struct {
	logger    Logger
	executors []*executor
	wg        sync.WaitGroup
	//shutdown  chan bool
}

func newRunner(logger Logger) runner {
	return runner{
		logger: logger,
		//shutdown: make(chan bool, 1),
	}
}

func (r *runner) run(commands []string, taskID string, debug bool) {
	r.executors = make([]*executor, len(commands))

	// nothing to run
	if len(commands) == 0 {
		return
	}

	log.Printf("runner: Running task: %s", taskID)
	r.sendLog(taskID, model.StageStart, false, model.UnixTime(), debug)
	defer func() { r.sendLog(taskID, model.StageEnd, false, model.UnixTime(), debug) }()

	// run in parallel and wait for them to finish
	for i, command := range commands {
		r.executors[i] = newExecutor(taskID, model.StageRun, r.logger, debug)
		r.wg.Add(1)
		go func(c string, e *executor) {
			defer r.wg.Done()
			e.execute(c)
		}(command, r.executors[i])
	}
	r.wg.Wait()

	log.Println("runner: All processes are ended.")
}

func (r *runner) sendLog(task, output string, error bool, time model.UnixTimeType, debug bool) {
	r.logger.Send(&model.Log{task, model.StageRun, model.CommandByAgent, output, error, time, debug})
}

func (r *runner) stop() bool {
	if len(r.executors) == 0 {
		return true
	}
	log.Println("runner: Shutting down the runner...")
	var success bool
	for i := 0; i < len(r.executors); i++ {
		success = r.executors[i].stop()
	}

	//<-r.shutdown // wait for all logs
	log.Println("runner: Success:", success)
	return success
}
