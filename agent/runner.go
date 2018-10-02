package main

import (
	"fmt"
	"log"
	"os"
	"sync"

	"code.linksmart.eu/dt/deployment-tool/agent/buffer"
	"code.linksmart.eu/dt/deployment-tool/model"
)

type runner struct {
	logCollector
	buf       buffer.Buffer
	executors []*executor
	wg        sync.WaitGroup
}

func newRunner(lc logCollector) runner {
	return runner{
		logCollector: lc,
		buf:          buffer.NewBuffer(RunBufferCapacity),
	}
}

func (r *runner) run(commands []string, taskID string) {
	r.executors = make([]*executor, len(commands))

	// nothing to run
	if len(commands) == 0 {
		return
	}

	log.Printf("run() Running task: %s", taskID)
	r.sendRunResponse(model.ResponseLog, taskID, "")

	wd, _ := os.Getwd()
	wd = fmt.Sprintf("%s/tasks/%s", wd, taskID)

	resCh := make(chan model.Response)
	go func() {
		execError := false
		for res := range resCh {
			if res.Error {
				execError = true
				r.sendRunResponse(model.ResponseError, taskID, res.Output)
			}
			r.buf.Insert(res)
			log.Printf("run() %v", res)
		}

		log.Printf("run() closing collector routine.")

		if !execError {
			r.sendRunResponse(model.ResponseSuccess, taskID, "")
		}
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

func (r *runner) sendRunResponse(status model.ResponseType, taskID, message string) {
	var response []model.Response
	if message != "" || status == model.ResponseError {
		response = []model.Response{{Output: message, Error: status == model.ResponseError}}
	}
	r.sendResponse(&model.BatchResponse{
		Stage:        model.StageRun,
		ResponseType: status,
		TaskID:       taskID,
		Responses:    response,
	})
}

func (r *runner) stop() bool {
	log.Println("Shutting down the runner...")
	var success bool
	for i := 0; i < len(r.executors); i++ {
		success = r.executors[i].stop()
	}
	r.wg.Wait() // wait for pending runs
	r.buf.Flush()
	return success
}
