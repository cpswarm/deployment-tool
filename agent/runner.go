package main

import (
	"log"
	"sync"

	"code.linksmart.eu/dt/deployment-tool/manager/model"
)

type runner struct {
	logEnqueue enqueueFunc
	executors  []*executor
	wg         sync.WaitGroup
}

func newRunner(logEnqueue enqueueFunc) runner {
	return runner{
		logEnqueue: logEnqueue,
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

	successCh := make(chan bool, len(commands))
	// run in parallel and wait for them to finish
	for i, command := range commands {
		r.executors[i] = newExecutor(taskID, model.StageRun, r.logEnqueue, debug)
		r.wg.Add(1)
		go func(c string, e *executor) {
			defer r.wg.Done()
			successCh <- e.execute(c)
		}(command, r.executors[i])
	}
	r.wg.Wait()
	close(successCh)

	var endErr bool
	for success := range successCh {
		if !success {
			endErr = true
			break
		}
	}

	r.sendLog(taskID, model.StageEnd, endErr, debug)
	log.Println("runner: All processes are ended.")
}

func (r *runner) sendLog(task, output string, error bool, debug bool) {
	r.logEnqueue(&model.Log{task, model.StageRun, model.CommandByAgent, output, error, model.UnixTime(), debug})
}

func (r *runner) stop() (success bool) {
	if len(r.executors) == 0 {
		return true
	}
	log.Println("runner: Shutting down...")
	success = true
	for i := range r.executors {
		if !r.executors[i].stop() {
			success = false
		}
	}
	log.Println("runner: Shutdown success:", success)
	return success
}
