package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"syscall"

	"code.linksmart.eu/dt/deployment-tool/manager/model"
	"code.linksmart.eu/dt/deployment-tool/manager/source"
)

type executor struct {
	workDir    string
	task       string
	stage      string
	logEnqueue logQueuer
	cmd        *exec.Cmd
	debug      bool
}

func newExecutor(task, stage string, logEnqueue logQueuer, debug bool) *executor {
	var wd string
	if task == model.TaskTerminal {
		wd = fmt.Sprintf("%s/%s", WorkDir, TerminalDir)
	} else {
		wd = fmt.Sprintf("%s/tasks/%s", WorkDir, task)
		wd += "/" + source.ExecDir(wd)
	}

	// force Python std streams to be unbuffered
	os.Setenv("PYTHONUNBUFFERED", "1")

	return &executor{
		workDir:    wd,
		task:       task,
		stage:      stage,
		logEnqueue: logEnqueue,
		debug:      debug,
	}
}

// execute executes a command
func (e *executor) execute(command string) (success bool) {
	e.sendLog(command, model.ExecStart, false)

	bashCommand := []string{"/bin/sh", "-c", command}
	e.cmd = exec.Command(bashCommand[0], bashCommand[1:]...)

	e.cmd.Dir = e.workDir
	e.cmd.SysProcAttr = &syscall.SysProcAttr{}
	e.cmd.SysProcAttr.Setsid = true

	outStream, err := e.cmd.StdoutPipe()
	if err != nil {
		e.sendLogFatal(command, err.Error())
		return false
	}

	errStream, err := e.cmd.StderrPipe()
	if err != nil {
		e.sendLogFatal(command, err.Error())
		return false
	}

	// stdout reader
	go func(stream io.ReadCloser) {
		scanner := bufio.NewScanner(stream)

		for scanner.Scan() {
			e.sendLog(command, scanner.Text(), false)
		}
		if err = scanner.Err(); err != nil {
			e.sendLog(command, err.Error(), true)
			log.Println("executor: Error:", err)
		}
		stream.Close()
	}(outStream)

	// stderr reader
	go func(stream io.ReadCloser) {
		scanner := bufio.NewScanner(stream)

		for scanner.Scan() {
			e.sendLog(command, scanner.Text(), true)
		}
		if err = scanner.Err(); err != nil {
			e.sendLog(command, err.Error(), true)
			log.Println("executor: Error:", err)
		}
		stream.Close()
	}(errStream)

	err = e.cmd.Run()
	if err != nil {
		e.sendLogFatal(command, err.Error())
		return false
	}
	e.sendLog(command, model.ExecEnd, false)
	e.cmd = nil
	return true
}

func (e *executor) sendLog(command, output string, error bool) {
	e.logEnqueue(&model.Log{e.task, e.stage, command, output, error, model.UnixTime(), e.debug})
}

func (e *executor) sendLogFatal(command, output string) {
	log.Println("executor: Error:", output)
	e.sendLog(command, output, true)
	e.sendLog(command, model.ExecEnd, true)
}

func (e *executor) stop() (success bool) {
	if e.cmd == nil || e.cmd.Process == nil {
		return true
	}
	pid := e.cmd.Process.Pid

	err := e.cmd.Process.Signal(syscall.SIGTERM)
	if err != nil {
		log.Printf("executor: Error terminating process %d: %s", pid, err)
		return false
	}
	err = e.cmd.Process.Release()
	if err != nil {
		log.Printf("executor: Error releasing process %d: %s", pid, err)
	} else {
		log.Println("executor: Terminated process:", pid)
		return true
	}

	err = e.cmd.Process.Kill()
	if err != nil {
		log.Printf("executor: Error killing process %d: %s", pid, err)
		return false
	}
	log.Println("executor: Killed process:", pid)
	return true
}
