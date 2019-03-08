package main

import (
	"log"

	"code.linksmart.eu/dt/deployment-tool/manager/model"
)

type installer struct {
	logger   Logger
	executor *executor
}

func newInstaller(logger Logger) installer {
	return installer{
		logger: logger,
	}
}

func (i *installer) install(commands []string, mode, taskID string, debug bool) bool {
	//i.sendLog(mode, taskID, model.StageStart, false, debug)

	// nothing to execute
	if len(commands) == 0 {
		log.Printf("installer: Nothing to execute.")
		//i.sendLog(mode, taskID, model.StageEnd, false, debug)
		return true
	}

	log.Printf("installer: Installing task: %s", taskID)

	// execute sequentially, return if one fails
	i.executor = newExecutor(taskID, mode, i.logger, debug)
	for _, command := range commands {
		success := i.executor.execute(command)
		if !success {
			i.sendLogFatal(mode, taskID, "ended with errors")
			return false
		}
	}

	log.Printf("installer: Install ended.")
	if mode == model.StageInstall {
		i.sendLog(mode, taskID, model.StageEnd, false, debug)
	}
	return true
}

func (i *installer) sendLog(mode, task, output string, error bool, debug bool) {
	i.logger.Send(&model.Log{task, mode, model.CommandByAgent, output, error, model.UnixTime(), debug})
}

func (i *installer) sendLogFatal(mode, task, output string) {
	log.Printf("installer: %s", output)
	if output != "" {
		i.sendLog(mode, task, output, true, true)
	}
	i.sendLog(mode, task, model.StageEnd, true, true)
}

func (r *installer) stop() bool {
	log.Println("installer: Shutting down...")
	success := r.executor.stop()
	return success
}
