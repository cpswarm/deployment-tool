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
	buf     buffer.Buffer
	runners []*executor
	wg      sync.WaitGroup
}

func newRunner() *runner {
	return &runner{
		buf: buffer.NewBuffer(LogBufferCapacity),
	}
}

func (r *runner) run(commands []string, taskID string) {
	// stop existing runners
	r.stop()
	r.runners = make([]*executor, len(commands))

	// nothing to run
	if len(commands) == 0 {
		return
	}

	log.Printf("run() Running task: %s", taskID)

	wd, _ := os.Getwd()
	wd = fmt.Sprintf("%s/tasks/%s", wd, taskID)

	resCh := make(chan model.Response)
	go func() {
		for res := range resCh {
			r.buf.Insert(res)
			log.Printf("run() %v", res)
		}
		log.Printf("run() closing collector routine.")
	}()

	// run in parallel and wait for them to finish
	for i := 0; i < len(commands); i++ {
		r.runners[i] = newExecutor(wd)
		r.wg.Add(1)
		go r.runners[i].executeAndCollectWg([]string{commands[i]}, resCh, &r.wg)
	}
	r.wg.Wait()

	close(resCh)
	log.Println("run() All processes are ended.")
}

func (r *runner) stop() {
	log.Println("Shutting down the runner...")
	for i := 0; i < len(r.runners); i++ {
		r.runners[i].stop()
	}
	r.wg.Wait() // wait for pending runs
	r.buf.Flush()
}
