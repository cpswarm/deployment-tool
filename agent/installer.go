package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"code.linksmart.eu/dt/deployment-tool/agent/buffer"
	"code.linksmart.eu/dt/deployment-tool/model"
	"github.com/mholt/archiver"
	"github.com/pbnjay/memory"
)

type installer struct {
	logCollector
	buf      buffer.Buffer
	executor *executor
}

func newInstaller(lc logCollector) installer {
	return installer{
		logCollector: lc,
		buf:          buffer.NewBuffer(InstallBufferCapacity),
	}
}

func (i *installer) evaluate(ta model.TaskAnnouncement) bool {
	sizeLimit := memory.TotalMemory() / 2 // TODO calculate this based on the available memory
	return ta.Size <= sizeLimit
}

func (i *installer) store(artifacts []byte, taskID string) {
	// set work directory
	wd, _ := os.Getwd()
	wd = fmt.Sprintf("%s/tasks", wd)
	taskDir := fmt.Sprintf("%s/%s", wd, taskID)
	log.Println("Task work directory:", taskDir)

	// decompress and store
	log.Printf("Deploying %d bytes of artifacts.", len(artifacts))
	err := archiver.TarGz.Read(bytes.NewBuffer(artifacts), taskDir)
	if err != nil {
		log.Fatal(err) // TODO send client error
	}
	artifacts = nil // release memory
}

func (i *installer) install(commands []string, taskID string) bool {
	// nothing to execute
	if len(commands) == 0 {
		return true
	}

	log.Printf("install() Installing task: %s", taskID)

	// execute and collect results
	resCh := make(chan model.BatchResponse)
	go func() {
		for res := range resCh {
			res.TaskID = taskID
			res.Stage = model.StageInstall
			i.sendResponse(&res)
		}
	}()

	logOpt := model.Log{
		Interval:  "3s",
		Verbosity: "INFO",
	}

	wd, _ := os.Getwd()
	wd = fmt.Sprintf("%s/tasks/%s", wd, taskID)

	exec := newExecutor(wd)

	success := exec.executeAndCollectBatch(commands, logOpt, resCh)

	log.Println("install() Installation ended.")
	return success
}

// clean removed old task directory
func (i *installer) clean(taskID string) {
	log.Println("clean()", taskID)
	wd, _ := os.Getwd()
	wd = fmt.Sprintf("%s/tasks", wd)

	_, err := os.Stat(wd)
	if err != nil && os.IsNotExist(err) {
		// nothing to remove
		return
	}
	files, err := ioutil.ReadDir(wd)
	if err != nil {
		log.Printf("Error reading work dir: %s", err)
		return
	}
	for i := 0; i < len(files); i++ {
		if files[i].Name() != taskID {
			log.Println(files[i].Name(), taskID)
			filename := fmt.Sprintf("%s/%s", wd, files[i].Name())
			log.Printf("Removing: %s", filename)
			err = os.RemoveAll(filename)
			if err != nil {
				log.Printf("Error removing: %s", err)
			}
		}
	}
}

func (r *installer) stop() bool {
	log.Println("Shutting down the runner...")
	success := r.executor.stop()
	r.buf.Flush()
	return success
}
