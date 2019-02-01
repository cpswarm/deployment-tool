package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"reflect"
	"strings"

	"code.linksmart.eu/dt/deployment-tool/manager/model"
	"code.linksmart.eu/dt/deployment-tool/manager/source"
	"github.com/satori/go.uuid"
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

func (a *agent) loadConf() {
	err := a.loadState()
	if err != nil {
		log.Printf("Error loading state file: %s. Starting fresh.", DefaultStateFile)
	}

	// LOAD AND REPLACE WITH ENV VARIABLES
	var changed bool

	id := os.Getenv("ID")
	if id == "" && a.target.AutoGenID == "" {
		a.target.AutoGenID = uuid.NewV4().String()
		log.Println("Generated target ID:", a.target.AutoGenID)
		a.target.ID = a.target.AutoGenID
		changed = true
	} else if id == "" && a.target.ID != a.target.AutoGenID {
		log.Println("Taking previously generated ID:", a.target.AutoGenID)
		a.target.ID = a.target.AutoGenID
		changed = true
	} else if id != "" && id != a.target.ID {
		log.Println("Taking ID from env var:", id)
		a.target.ID = id
		changed = true
	}

	var tags []string
	tagsString := os.Getenv("TAGS")
	if tagsString != "" {
		tags = strings.Split(tagsString, ",")
		for i := 0; i < len(tags); i++ {
			tags[i] = strings.TrimSpace(tags[i])
		}
	}
	if !reflect.DeepEqual(tags, a.target.Tags) {
		a.target.Tags = tags
		changed = true
	}

	if changed {
		a.saveState()
	}
}

func (a *agent) loadState() error {
	if _, err := os.Stat(DefaultStateFile); os.IsNotExist(err) {
		return err
	}

	b, err := ioutil.ReadFile(DefaultStateFile)
	if err != nil {
		return fmt.Errorf("error reading state file: %s", err)
	}

	err = json.Unmarshal(b, &a.target)
	if err != nil {
		return fmt.Errorf("error parsing state file: %s", err)
	}

	log.Println("Loaded state file:", DefaultStateFile)
	return nil
}


func (a *agent) saveState() {
	a.Lock()
	defer a.Unlock()

	b, _ := json.MarshalIndent(&a.target, "", "\t")
	err := ioutil.WriteFile(DefaultStateFile, b, 0600)
	if err != nil {
		log.Printf("Error saving state: %s", err)
		return
	}
	log.Println("Saved state:", DefaultStateFile)
}