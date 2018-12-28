package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"code.linksmart.eu/dt/deployment-tool/manager/model"
	"code.linksmart.eu/dt/deployment-tool/manager/source"
	"github.com/pbnjay/memory"
)

type installer struct {
	logger   chan<- model.Log
	executor *executor
}

func newInstaller(logger chan<- model.Log) installer {
	return installer{
		logger: logger,
	}
}

func (i *installer) evaluate(ann *model.Announcement) bool {
	sizeLimit := memory.TotalMemory() / 2 // TODO calculate this based on the available memory
	return uint64(ann.Size) <= sizeLimit
}

func (i *installer) store(artifacts []byte, dir string) {

	taskDir := fmt.Sprintf("%s/tasks/%s", WorkDir, dir)
	log.Println("installer: Task work directory:", taskDir)

	// nothing to store
	if len(artifacts) == 0 {
		log.Printf("installer: Nothing to store.")
		// create task with source directory
		err := os.MkdirAll(fmt.Sprintf("%s/%s", taskDir, source.SourceDir), 0755)
		if err != nil {
			log.Printf("installer: Error creating source directory: %s", err)
		}
		return
	}

	err := os.Mkdir(taskDir, 0755)
	if err != nil {
		log.Printf("installer: Error creating task directory: %s", err)
	}

	// decompress and store
	log.Printf("installer: Deploying %d bytes of artifacts.", len(artifacts))
	err = model.DecompressFiles(artifacts, taskDir)
	if err != nil {
		log.Printf("installer: Error reading archive: %s", err) // TODO send to manager
	}
	artifacts = nil // release memory
}

func (i *installer) install(commands []string, taskID string, debug bool) bool {
	// nothing to execute
	if len(commands) == 0 {
		log.Printf("installer: Nothing to execute.")
		return true
	}

	log.Printf("installer: Installing task: %s", taskID)
	i.sendLog(taskID, "", model.StageStart, false, model.UnixTime(), debug)
	defer func() { i.sendLog(taskID, "", model.StageEnd, false, model.UnixTime(), debug) }()

	// execute sequentially, return if one fails
	i.executor = newExecutor(taskID, model.StageInstall, i.logger, debug)
	for _, command := range commands {
		success := i.executor.execute(command)
		if !success {
			log.Printf("installer: Ended due to error.")
			return false
		}
	}

	log.Printf("installer: Install ended.")
	return true
}

func (i *installer) sendLog(task, command, output string, error bool, time model.UnixTimeType, debug bool) {
	i.logger <- model.Log{task, model.StageInstall, command, output, error, time, debug}
}

// clean removed old task directory
func (i *installer) clean(taskID string) {
	log.Println("installer: Removing files for task:", taskID)

	wd := fmt.Sprintf("%s/tasks", WorkDir)

	_, err := os.Stat(wd)
	if err != nil && os.IsNotExist(err) {
		// nothing to remove
		return
	}
	files, err := ioutil.ReadDir(wd)
	if err != nil {
		log.Printf("installer: Error reading work dir: %s", err)
		return
	}
	for i := 0; i < len(files); i++ {
		if files[i].Name() != taskID {
			log.Println(files[i].Name(), taskID)
			filename := fmt.Sprintf("%s/%s", wd, files[i].Name())
			log.Printf("installer: Removing: %s", filename)
			err = os.RemoveAll(filename)
			if err != nil {
				log.Printf("installer: Error removing: %s", err)
			}
		}
	}
}

func (r *installer) stop() bool {
	log.Println("installer: Shutting down...")
	success := r.executor.stop()
	return success
}
