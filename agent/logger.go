package main

import (
	"encoding/json"
	"log"
	"time"

	"code.linksmart.eu/dt/deployment-tool/agent/buffer"
	"code.linksmart.eu/dt/deployment-tool/manager/model"
)

const (
	BufferCapacity = 255
	LogInterval    = 5 * time.Second
)

type Logger interface {
	Report(model.LogRequest)
	Writer() chan<- model.Log
}

type logger struct {
	// options
	targetID string
	//taskID   string
	debug bool

	buffer     buffer.Buffer
	queue      chan model.Log
	ticker     *time.Ticker
	tickerQuit chan struct{}

	responseCh chan<- model.Message
}

func NewLogger(targetID string, debug bool, responseCh chan<- model.Message) Logger {
	l := &logger{
		targetID:   targetID,
		debug:      debug,
		responseCh: responseCh,
		buffer:     buffer.NewBuffer(BufferCapacity),
		tickerQuit: make(chan struct{}),
		queue:      make(chan model.Log),
	}

	go l.startTicker()

	return l
}

func (l *logger) Report(request model.LogRequest) {
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

func (l *logger) Writer() chan<- model.Log {
	return l.queue
}

func (l *logger) Stop() {
	if l.ticker != nil {
		l.ticker.Stop()
		close(l.tickerQuit)
		l.tickerQuit = make(chan struct{})
	}
}

func (l *logger) startTicker() {
	l.ticker = time.NewTicker(LogInterval)
	var tickBuffer []model.Log
	for {

		select {
		case logM := <-l.queue:
			if evalEnv(EnvDebug) {
				if logM.Error {
					log.Println("logger: Err:", logM.Output)
				} else {
					log.Println("logger: Log:", logM.Output)
				}
			}
			// keep everything in memory (FIFO)
			l.buffer.Insert(logM)
			// buffer everything when in debug mode, otherwise just state info
			if logM.Debug ||
				logM.Output == model.StageStart || logM.Output == model.StageEnd ||
				logM.Output == model.ExecStart || logM.Output == model.ExecEnd {
				tickBuffer = append(tickBuffer, logM)
			}
		case <-l.ticker.C:
			// send out and flush
			if len(tickBuffer) > 0 {
				l.send(tickBuffer, false)
				tickBuffer = nil
			}
		case <-l.tickerQuit:
			// send out and flush
			if len(tickBuffer) > 0 {
				l.send(tickBuffer, false)
				tickBuffer = nil
			}
			log.Println("logger: Quit ticker")
			return
		}
	}
}

func (l *logger) send(logs []model.Log, onRequest bool) {
	log.Printf("logger: Sending %d entries.", len(logs))
	b, _ := json.Marshal(model.Response{
		TargetID:  l.targetID,
		Logs:      logs,
		OnRequest: onRequest,
	})
	l.responseCh <- model.Message{string(model.ResponseLog), b}
}
