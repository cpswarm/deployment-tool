package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"code.linksmart.eu/dt/deployment-tool/manager/model"
	"code.linksmart.eu/dt/deployment-tool/manager/source"
)

func (a *agent) sendLog(task, output string, error bool, debug bool) {
	a.logger.Send(&model.Log{task, model.StageTransfer, model.CommandByAgent, output, error, model.UnixTime(), debug})
}

func (a *agent) sendLogFatal(task, output string) {
	log.Printf("transfer: %s", output)
	if output != "" {
		a.sendLog(task, output, true, true)
	}
	a.sendLog(task, model.StageEnd, true, true)
}

func (a *agent) saveArtifacts(artifacts []byte, taskID string, debug bool) error {
	defer func() {
		artifacts = nil // release memory
	}()

	taskDir := fmt.Sprintf("%s/tasks/%s", WorkDir, taskID)
	log.Println("Task work directory:", taskDir)

	// nothing to store
	if len(artifacts) == 0 {
		log.Printf("installer: Nothing to store.")
		// create task with source directory
		err := os.MkdirAll(fmt.Sprintf("%s/%s", taskDir, source.SourceDir), 0755)
		if err != nil {
			return fmt.Errorf("error creating source directory: %s", err)
		}
		return nil
	}

	// create task directory
	err := os.MkdirAll(taskDir, 0755)
	if err != nil {
		return fmt.Errorf("error creating task directory: %s", err)
	}

	// decompress and store
	log.Printf("installer: Deploying %d bytes of artifacts.", len(artifacts))
	err = model.DecompressFiles(artifacts, taskDir)
	if err != nil {
		return fmt.Errorf("error reading archive: %s", err)
	}
	a.sendLog(taskID, fmt.Sprintf("decompressed archive of %d bytes", len(artifacts)), false, debug)

	return nil
}

// removeOtherTasks removed old task directory
func (*agent) removeOtherTasks(taskID string) {
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
