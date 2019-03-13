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
}

func newRunner(logger Logger) runner {
	return runner{
		logger: logger,
	}
}

func (r *runner) run(commands []string, taskID string, debug bool) {
	r.executors = make([]*executor, len(commands))

	// nothing to run
	if len(commands) == 0 {
		return
	}

	log.Printf("runner: Running task: %s", taskID)
	r.sendLog(taskID, model.StageStart, false, debug)

	errCh := make(chan bool, len(commands))
	// run in parallel and wait for them to finish
	for i, command := range commands {
		r.executors[i] = newExecutor(taskID, model.StageRun, r.logger, debug)
		r.wg.Add(1)
		go func(c string, e *executor) {
			defer r.wg.Done()
			errCh <- e.execute(c)
		}(command, r.executors[i])
	}
	r.wg.Wait()
	close(errCh)

	var endErr bool
	for err := range errCh {
		if err {
			endErr = true
			break
		}
	}

	r.sendLog(taskID, model.StageEnd, endErr, debug)
	log.Println("runner: All processes are ended.")
}

func (r *runner) sendLog(task, output string, error bool, debug bool) {
	r.logger.Send(&model.Log{task, model.StageRun, model.CommandByAgent, output, error, model.UnixTime(), debug})
}

func (r *runner) stop() (success bool) {
	if len(r.executors) == 0 {
		return true
	}
	log.Println("runner: Shutting down the runner...")
	success = true
	for i := range r.executors {
		if !r.executors[i].stop() {
			success = false
		}
	}
	log.Println("runner: Shutdown success:", success)
	return success
}
