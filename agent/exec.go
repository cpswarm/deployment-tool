package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"sync"
	"syscall"

	"code.linksmart.eu/dt/deployment-tool/manager/model"
	"code.linksmart.eu/dt/deployment-tool/manager/source"
)

type executor struct {
	workDir    string
	task       string
	stage      string
	logEnqueue enqueueFunc
	cmd        *exec.Cmd
	debug      bool
}

func newExecutor(task, stage string, logEnqueue enqueueFunc, debug bool) *executor {
	var wd string
	if task == model.TaskTerminal {
		wd = fmt.Sprintf("%s/%s", WorkDir, TerminalDir)
	} else {
		wd = fmt.Sprintf("%s/tasks/%s", WorkDir, task)
		dir, _ := source.ExecDir(wd)
		wd += "/" + dir
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

	e.cmd = exec.Command("/bin/sh", "-c", command)

	defer func() { e.cmd = nil }()

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

	var wg sync.WaitGroup

	// stdout reader
	wg.Add(1)
	go func(stream io.ReadCloser) {
		scanner := bufio.NewScanner(stream)
		for scanner.Scan() {
			e.sendLog(command, scanner.Text(), false)
		}
		wg.Done()
	}(outStream)

	// stderr reader
	wg.Add(1)
	go func(stream io.ReadCloser) {
		scanner := bufio.NewScanner(stream)
		for scanner.Scan() {
			e.sendLog(command, scanner.Text(), true)
		}
		wg.Done()
	}(errStream)

	err = e.cmd.Run()
	if err != nil {
		e.sendLogFatal(command, err.Error())
		return false
	}
	wg.Wait()
	e.sendLog(command, model.ExecEnd, false)
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
	defer func() { e.cmd = nil }()

	pid := e.cmd.Process.Pid

	err := e.cmd.Process.Signal(syscall.SIGINT)
	if err != nil {
		log.Printf("executor: Error interrupting process %d: %s", pid, err)
		return false
	}
	err = e.cmd.Process.Release()
	if err != nil {
		log.Printf("executor: Error releasing process %d: %s", pid, err)
	} else {
		log.Println("executor: Interrupted process:", pid)
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
