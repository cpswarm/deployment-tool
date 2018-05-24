package main

import (
	"bufio"
	"bytes"
	"io"
	"log"
	"os/exec"
	"sync/atomic"
	"syscall"
	"time"

	"code.linksmart.eu/dt/deployment-tool/model"
	"github.com/mholt/archiver"
)

func (a *agent) storeArtifacts(wd string, b []byte) {
	log.Printf("Deploying %d bytes of artifacts.", len(b))
	err := archiver.TarGz.Read(bytes.NewBuffer(b), wd)
	if err != nil {
		log.Fatal(err)
	}
}

func (a *agent) responseBatchCollector(task *model.Task, wd string, interval time.Duration, out chan model.BatchResponse) {
	resCh := make(chan model.Response)

	go responseCollector(task.Commands, wd, resCh)

	batch := model.BatchResponse{
		ResponseType: model.ResponseLog,
		TaskID:       task.ID,
		TargetID:     a.ID,
	}
	var foundError bool

	ticker := time.NewTicker(interval)
LOOP:
	for {
		select {
		case res, open := <-resCh:
			if !open {
				break LOOP
			}
			log.Printf("[res] %+v", res)
			if len(res.Stderr) > 0 {
				batch.ResponseType = model.ResponseError
			}
			//log.Printf("%s -- %d -- %s -- %s -- %f", res.Command, res.LineNum, res.Stdout, res.Stderr, res.TimeElapsed)
			batch.Responses = append(batch.Responses, res)
			batch.TimeElapsed = res.TimeElapsed
		case <-ticker.C:
			//out <- batch
			a.sendResponse(&batch)
			//log.Printf("Batch: %+v", batch)

			// flush responses
			batch.Responses = []model.Response{}
		}
	}

	//out <- batch
	a.sendResponse(&batch)
	log.Printf("Final Batch: %+v", batch)
	if !foundError {
		batch.ResponseType = model.ResponseComplete
	}
}

func responseCollector(commands []string, wd string, out chan model.Response) {
	start := time.Now()

	stdout, stderr := make(chan logLine), make(chan logLine)
	callback := make(chan error)

	go executeMultiple(commands, wd, stdout, stderr, callback)

	for open := true; open; {
		select {
		case x := <-stdout:
			out <- model.Response{Command: x.command, Stdout: x.line, LineNum: x.lineNum, TimeElapsed: time.Since(start).Seconds()}
		case x := <-stderr:
			out <- model.Response{Command: x.command, Stderr: x.line, LineNum: x.lineNum, TimeElapsed: time.Since(start).Seconds()}
		case _, open = <-callback:
			// do nothing
		}
	}

	//log.Println("closing responseCollector")
	close(out)
}

func executeMultiple(commands []string, wd string, stdout, stderr chan logLine, callback chan error) {
	for _, command := range commands {
		execute(command, wd, stdout, stderr)
	}
	close(callback)
}

// one line of log for a command
type logLine struct {
	command string
	line    string
	lineNum uint32
}

func execute(command, wd string, stdout, stderr chan logLine) {
	bashCommand := []string{"/bin/bash", "-c", command}
	cmd := exec.Command(bashCommand[0], bashCommand[1:]...)

	cmd.Dir = wd
	cmd.SysProcAttr = &syscall.SysProcAttr{}
	cmd.SysProcAttr.Setsid = true

	var line uint32

	outStream, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}

	errStream, err := cmd.StderrPipe()
	if err != nil {
		log.Fatal(err)
	}

	// stdout reader
	go func(stream io.ReadCloser) {
		scanner := bufio.NewScanner(stream)

		for scanner.Scan() {
			atomic.AddUint32(&line, 1)
			//log.Println(scanner.Text())
			stdout <- logLine{command, scanner.Text(), line}
		}
		if err = scanner.Err(); err != nil {
			stderr <- logLine{command, err.Error(), line}
			log.Println("Error:", err)
		}
		stream.Close()
	}(outStream)

	// stderr reader
	go func(stream io.ReadCloser) {
		scanner := bufio.NewScanner(stream)

		for scanner.Scan() {
			atomic.AddUint32(&line, 1)
			//log.Println("stderr:", scanner.Text())
			stderr <- logLine{command, scanner.Text(), line}
		}
		if err = scanner.Err(); err != nil {
			stderr <- logLine{command, err.Error(), line}
			log.Println("Error:", err)
		}
		stream.Close()
	}(errStream)

	//defer log.Println("closing execute")

	err = cmd.Run()
	if err != nil {
		atomic.AddUint32(&line, 1)
		stderr <- logLine{command, err.Error(), line}
		return
	}
	atomic.AddUint32(&line, 1)
	stdout <- logLine{command, "exit status 0", line}

}
