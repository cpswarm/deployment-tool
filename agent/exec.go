package main

import (
	"bufio"
	"bytes"
	"io"
	"log"
	"os/exec"
	"sync"
	"sync/atomic"
	"syscall"

	"code.linksmart.eu/dt/deployment-tool/model"
	"github.com/mholt/archiver"
)

type executor struct {
	workDir string
	cmd     *exec.Cmd
}

func newExecutor(workDir string) *executor {
	return &executor{workDir: workDir}
}

func (e *executor) storeArtifacts(b []byte) {
	log.Printf("Deploying %d bytes of artifacts.", len(b))
	err := archiver.TarGz.Read(bytes.NewBuffer(b), e.workDir)
	if err != nil {
		log.Fatal(err)
	}
}

// executeAndCollectWg executes multiple commands and uses a wait group to signal the completion
// NOTE: unlike executeAndCollect, this function does not close the channel upon completion
func (e *executor) executeAndCollectWg(commands []string, out chan model.Log, wg *sync.WaitGroup) {

	stdout, stderr := make(chan logLine), make(chan logLine)
	callback := make(chan error)

	go e.executeMultiple(commands, stdout, stderr, callback)

	defer wg.Done()
	for {
		select {
		case x := <-stdout:
			out <- model.Log{Command: x.command, Output: x.line, LineNum: x.lineNum, Time: x.time}
		case x := <-stderr:
			out <- model.Log{Command: x.command, Output: x.line, LineNum: x.lineNum, Time: x.time, Error: true}
		case <-callback:
			return
		}
	}
}

// executeAndCollect executes multiple commands and closes the channel upon completion
func (e *executor) executeAndCollect(commands []string, out chan model.Log) {

	stdout, stderr := make(chan logLine), make(chan logLine)
	callback := make(chan error)

	go e.executeMultiple(commands, stdout, stderr, callback)

	defer close(out)
	for {
		select {
		case x := <-stdout:
			out <- model.Log{Command: x.command, Output: x.line, LineNum: x.lineNum, Time: x.time}
		case x := <-stderr:
			out <- model.Log{Command: x.command, Output: x.line, LineNum: x.lineNum, Time: x.time, Error: true}
		case <-callback:
			return
		}
	}
}

// executeMultiple sequentially executes multiple commands
func (e *executor) executeMultiple(commands []string, stdout, stderr chan logLine, callback chan error) {
	for _, command := range commands {
		e.execute(command, stdout, stderr)
	}
	close(callback) // TODO use callback to return exit errors
}

// one line of log for a command
type logLine struct {
	command string
	line    string
	lineNum uint32
	time    model.UnixTimeType
}

// execute executes a command
func (e *executor) execute(command string, stdout, stderr chan logLine) {
	bashCommand := []string{"/bin/sh", "-c", command}
	e.cmd = exec.Command(bashCommand[0], bashCommand[1:]...)

	e.cmd.Dir = e.workDir
	e.cmd.SysProcAttr = &syscall.SysProcAttr{}
	e.cmd.SysProcAttr.Setsid = true

	var line uint32

	outStream, err := e.cmd.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}

	errStream, err := e.cmd.StderrPipe()
	if err != nil {
		log.Fatal(err)
	}

	// stdout reader
	go func(stream io.ReadCloser) {
		scanner := bufio.NewScanner(stream)

		for scanner.Scan() {
			atomic.AddUint32(&line, 1)
			//log.Println(scanner.Text())
			stdout <- logLine{command, scanner.Text(), line, model.UnixTime()}
		}
		if err = scanner.Err(); err != nil {
			stderr <- logLine{command, err.Error(), line, model.UnixTime()}
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
			stderr <- logLine{command, scanner.Text(), line, model.UnixTime()}
		}
		if err = scanner.Err(); err != nil {
			stderr <- logLine{command, err.Error(), line, model.UnixTime()}
			log.Println("Error:", err)
		}
		stream.Close()
	}(errStream)

	//defer log.Println("closing execute")

	err = e.cmd.Run()
	if err != nil {
		atomic.AddUint32(&line, 1)
		stderr <- logLine{command, err.Error(), line, model.UnixTime()}
		return
	}
	atomic.AddUint32(&line, 1)
	stdout <- logLine{command, "exit status 0", line, model.UnixTime()}
	e.cmd = nil
}

func (e *executor) stop() bool {
	if e.cmd == nil || e.cmd.Process == nil {
		return true
	}
	pid := e.cmd.Process.Pid

	err := e.cmd.Process.Signal(syscall.SIGTERM)
	if err != nil {
		log.Printf("Error terminating process %d: %s", pid, err)
		return false
	}
	err = e.cmd.Process.Release()
	if err != nil {
		log.Printf("Error releasing process %d: %s", pid, err)
	} else {
		log.Println("Terminated process:", pid)
		return true
	}

	err = e.cmd.Process.Kill()
	if err != nil {
		log.Printf("Error killing process %d: %s", pid, err)
		return false
	}
	log.Println("Killed process:", pid)
	return true
}
