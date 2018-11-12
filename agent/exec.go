package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"syscall"

	"code.linksmart.eu/dt/deployment-tool/model"
	"github.com/mholt/archiver"
)

type executor struct {
	workDir string
	task    string
	stage   string
	out     chan<- model.Log
	cmd     *exec.Cmd
	debug bool
}

func newExecutor(task, stage string, out chan<- model.Log, debug bool) *executor {
	wd, _ := os.Getwd()
	wd = fmt.Sprintf("%s/tasks/%s", wd, task)

	return &executor{
		workDir: wd,
		task:    task,
		stage:   stage,
		out:     out,
		debug : debug,
	}
}

func (e *executor) storeArtifacts(b []byte) {
	log.Printf("Deploying %d bytes of artifacts.", len(b))
	err := archiver.TarGz.Read(bytes.NewBuffer(b), e.workDir)
	if err != nil {
		log.Fatal(err)
	}
}

// execute executes a command
func (e *executor) execute(command string) bool {
	e.sendLog(command, model.ExecStart, false, model.UnixTime())

	bashCommand := []string{"/bin/sh", "-c", command}
	e.cmd = exec.Command(bashCommand[0], bashCommand[1:]...)

	e.cmd.Dir = e.workDir
	e.cmd.SysProcAttr = &syscall.SysProcAttr{}
	e.cmd.SysProcAttr.Setsid = true

	outStream, err := e.cmd.StdoutPipe()
	if err != nil {
		log.Println("Error:", err)
		e.sendLog(command, err.Error(), true, model.UnixTime())
		e.sendLog(command, model.ExecEnd, true, model.UnixTime())
		return false
	}

	errStream, err := e.cmd.StderrPipe()
	if err != nil {
		log.Println("Error:", err)
		e.sendLog(command, err.Error(), true, model.UnixTime())
		e.sendLog(command, model.ExecEnd, true, model.UnixTime())
		return false
	}

	// stdout reader
	go func(stream io.ReadCloser) {
		scanner := bufio.NewScanner(stream)

		for scanner.Scan() {
			//log.Println(scanner.Text())
			e.sendLog(command, scanner.Text(), false, model.UnixTime())
		}
		if err = scanner.Err(); err != nil {
			e.sendLog(command, err.Error(), true, model.UnixTime())
			log.Println("Error:", err)
		}
		stream.Close()
	}(outStream)

	// stderr reader
	go func(stream io.ReadCloser) {
		scanner := bufio.NewScanner(stream)

		for scanner.Scan() {
			//log.Println("stderr:", scanner.Text())
			e.sendLog(command, scanner.Text(), true, model.UnixTime())
		}
		if err = scanner.Err(); err != nil {
			e.sendLog(command, err.Error(), true, model.UnixTime())
			log.Println("Error:", err)
		}
		stream.Close()
	}(errStream)

	//defer log.Println("closing execute")

	err = e.cmd.Run()
	if err != nil {
		e.sendLog(command, err.Error(), true, model.UnixTime())
		e.sendLog(command, model.ExecEnd, true, model.UnixTime())
		log.Println("Error:", err)
		return false
	}
	e.sendLog(command, model.ExecEnd, false, model.UnixTime())
	e.cmd = nil
	return true
}

func (e *executor) sendLog(command, output string, error bool, time model.UnixTimeType) {
	e.out <- model.Log{e.task, e.stage, command, output, error, time, e.debug}
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
