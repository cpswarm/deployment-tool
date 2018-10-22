package main

import (
	"encoding/json"
	"log"
	"time"

	"code.linksmart.eu/dt/deployment-tool/agent/buffer"
	"code.linksmart.eu/dt/deployment-tool/model"
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
	// TODO sned logs after request.IfModifiedSince

	b, _ := json.Marshal(model.Response{
		TargetID: l.targetID,
		Logs: l.buffer.Collect(),
	})
	l.responseCh <- model.Message{model.ResponseLog, b}
	log.Printf("Reported logs.")
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
	var outBuffer []model.Log
	for {

		select {
		case logM := <-l.queue:
			log.Println("startTickert() add to q", logM)
			outBuffer = append(outBuffer, logM)
		case <-l.ticker.C:
			//log.Println("startTickert() tick", outBuffer) // TODO
			// send out and flush
			if len(outBuffer) > 0 {
				l.send(outBuffer)
				outBuffer = nil
			}
		case <-l.tickerQuit:
			// send out and flush
			if len(outBuffer) > 0 {
				l.send(outBuffer)
				outBuffer = nil
			}
			log.Println("Quit ticker")
			return
		}
	}
}

func (l *logger) send(logs []model.Log) {

	b, _ := json.Marshal(model.Response{
		TargetID: l.targetID,
		Logs: logs,
	})
	l.responseCh <- model.Message{string(model.ResponseLog), b}
	log.Printf("Sent logs for: %s", string(b))
}
