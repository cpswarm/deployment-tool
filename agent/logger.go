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
	LogWriter
	SetOpts(targetID, taskID string, debug bool)
	Report(stage model.StageType)
	Writer() LogWriter
}

type LogWriter interface {
	Insert(model.StageType, *model.Log)
	//Stop()
}

type logger struct {
	// options
	targetID string
	taskID   string
	debug    bool

	buffers    map[model.StageType]*buffer.Buffer
	queue      chan model.Response
	ticker     *time.Ticker
	tickerQuit chan struct{}

	responseCh chan<- model.Message
}

func NewLogger(responseCh chan<- model.Message) Logger {
	l := &logger{
		responseCh: responseCh,
		buffers:    make(map[model.StageType]*buffer.Buffer),
		tickerQuit: make(chan struct{}),
		queue:      make(chan model.Response),
	}
	l.buffers[model.StageTransfer] = buffer.NewBuffer(BufferCapacity)
	l.buffers[model.StageInstall] = buffer.NewBuffer(BufferCapacity)
	l.buffers[model.StageRun] = buffer.NewBuffer(BufferCapacity)

	go l.startTicker()

	return l
}

func (l *logger) SetOpts(targetID, taskID string, debug bool) {
	// TODO add mutex?
	l.targetID = targetID
	l.taskID = taskID
	l.debug = debug

	log.Printf("SetOpts() Task:%s Debug:%v", taskID, debug)
}

func (l *logger) Insert(stage model.StageType, logM *model.Log) {
	// keep everything in buffers
	l.buffers[stage].Insert(*logM)

	if l.debug || logM.Output == model.ProcessStart || logM.Output == model.ProcessExit || logM.Error {
		resp := model.Response{
			Stage: stage,
			Logs:  []model.Log{*logM},
		}
		log.Println("Insert", stage, logM)
		l.queue <- resp
	}
}

func (l *logger) Report(stage model.StageType) {

	b, _ := json.Marshal(model.Response{
		TargetID: l.targetID,
		TaskID:   l.taskID,
		Stage:    stage,
		Logs:     l.buffers[stage].Collect(),
	})
	l.responseCh <- model.Message{string(model.ResponseLog), b}
	log.Printf("Reported logs for: %s", stage)
}

func (l *logger) Writer() LogWriter {
	return l
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
	var stage model.StageType
	for {

		select {
		case res := <-l.queue:
			log.Println("startTickert() add to q", res)
			if stage == res.Stage {
				outBuffer = append(outBuffer, res.Logs...)
			} else {
				// send out and flush
				if len(outBuffer) > 0 {
					l.send(stage, outBuffer)
					outBuffer = nil
				}

				stage = res.Stage
				outBuffer = append(outBuffer, res.Logs...)
			}
		case <-l.ticker.C:
			//log.Println("startTickert() tick", outBuffer) // TODO
			// send out and flush
			if len(outBuffer) > 0 {
				l.send(stage, outBuffer)
				outBuffer = nil
			}
		case <-l.tickerQuit:
			// send out and flush
			if len(outBuffer) > 0 {
				l.send(stage, outBuffer)
				outBuffer = nil
			}
			log.Println("Quit ticker")
			return
		}
	}
}

func (l *logger) send(stage model.StageType, logs []model.Log) {

	b, _ := json.Marshal(model.Response{
		TargetID: l.targetID,
		TaskID:   l.taskID,
		Stage:    stage,
		Logs:     logs,
	})
	l.responseCh <- model.Message{string(model.ResponseLog), b}
	log.Printf("Sent logs for: %s", string(b))
}
