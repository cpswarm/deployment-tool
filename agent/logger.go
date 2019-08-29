package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"code.linksmart.eu/dt/deployment-tool/agent/buffer"
	"code.linksmart.eu/dt/deployment-tool/manager/env"
	"code.linksmart.eu/dt/deployment-tool/manager/model"
)

const (
	MemoryStorageCapacity  = 100             // number of logs kept in memory that can be queried
	OutgoingBufferCapacity = 255             // number of logs collected before the next flush timeout
	OutgoingFlushInterval  = 5 * time.Second // frequency of logs submissions to server
)

type logger struct {
	targetID   string
	responseCh chan<- model.Message

	buffer     buffer.Buffer
	queue      chan model.Log
	ticker     *time.Ticker
	tickerQuit chan struct{}
}

func newLogger(targetID string, responseCh chan<- model.Message) *logger {
	l := &logger{
		targetID:   targetID,
		responseCh: responseCh,
		buffer:     buffer.NewBuffer(MemoryStorageCapacity),
		tickerQuit: make(chan struct{}),
		queue:      make(chan model.Log),
	}

	go l.startTicker()

	return l
}

func (l *logger) startTicker() {
	l.ticker = time.NewTicker(OutgoingFlushInterval)
	tickBuffer := buffer.NewBuffer(OutgoingBufferCapacity)
	for {
		select {
		case logM := <-l.queue:
			if env.Debug {
				if logM.Error {
					log.Println("logger: Err:", logM.Output)
				} else {
					log.Println("logger: Log:", logM.Output)
				}
			}
			// keep everything in memory (FIFO)
			l.buffer.Insert(logM)
			// buffer everything when in debug mode, otherwise just errors and state info
			if logM.Debug ||
				logM.Error ||
				logM.Output == model.StageStart || logM.Output == model.StageEnd ||
				logM.Output == model.ExecStart || logM.Output == model.ExecEnd {
				tickBuffer.Insert(logM)
			}
		case <-l.ticker.C:
			// send out and flush
			if Connected() && tickBuffer.Size() > 0 {
				l.send(tickBuffer.Collect(), false)
				tickBuffer.Flush()
			}
		case <-l.tickerQuit:
			// send out and return
			if Connected() && tickBuffer.Size() > 0 {
				l.send(tickBuffer.Collect(), false)
			}
			return
		}
	}
}

type enqueueFunc func(*model.Log)

func (l *logger) enqueue(logM *model.Log) {
	l.queue <- *logM
}

func (l *logger) priorityEnqueue(logM *model.Log) {
	l.send([]model.Log{*logM}, false)
}

func (l *logger) send(logs []model.Log, onRequest bool) {
	log.Printf("logger: Sending %d entries.", len(logs))
	b, err := json.Marshal(model.Response{
		TargetID:  l.targetID,
		Logs:      logs,
		OnRequest: onRequest,
	})
	if err != nil {
		b = []byte(fmt.Sprintf("Error mashalling logs: %s", err))
		log.Printf("%s", b)
	}
	l.responseCh <- model.Message{string(model.ResponseLogs), b}
}

func (l *logger) report(request *model.LogRequest) {
	logs := l.buffer.Collect()
	// send logs since request.IfModifiedSince
	for i := range logs {
		if logs[i].Time >= request.IfModifiedSince {
			l.send(logs[i:], true)
			return
		}
	}
	log.Println("No logs since", request.IfModifiedSince)
}

func (l *logger) stop() {
	log.Println("logger: Shutting down...")
	if l.ticker != nil {
		l.ticker.Stop()
		close(l.tickerQuit)
	}
	log.Println("logger: Stopped")
}
