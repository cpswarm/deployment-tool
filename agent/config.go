package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"code.linksmart.eu/dt/deployment-tool/manager/model"
	"code.linksmart.eu/dt/deployment-tool/manager/zeromq"
	uuid "github.com/satori/go.uuid"
)

type target struct {
	mutex sync.Mutex
	model.TargetBase
	AutoGenID        string       `json:"autoID,omitempty"`
	Registered       bool         `json:"registered"`
	ZeromqServerConf zeromqServer `json:"zeromqServer"`
	ManagerAddr      string       `json:"-"`
	// active task
	TaskID             string           `json:"taskID"`
	TaskDebug          bool             `json:"taskDebug,omitempty"`
	TaskRun            []string         `json:"taskRun,omitempty"`
	TaskRunAutoRestart bool             `json:"taskRunAutoRestart,omitempty"`
	TaskHistory        map[string]uint8 `json:"taskHistory,omitempty"`
}

type zeromqServer struct {
	model.ZeromqServer
	host string
}

func loadConf() (*target, error) {

	t, err := loadState()
	if err != nil {
		log.Printf("Error loading state file: %s. Starting fresh.", DefaultStateFile)
		t = &target{}
	}
	if t.TaskHistory == nil {
		t.TaskHistory = make(map[string]uint8)
	}

	if os.Getenv(EnvManagerAddr) == "" {
		return nil, fmt.Errorf("manager address not set")
	}
	addr, err := url.Parse(os.Getenv(EnvManagerAddr))
	if err != nil {
		return nil, fmt.Errorf("error parsing manager address: %s", err)
	}
	t.ManagerAddr = addr.String()
	log.Println("Manager addr:", t.ManagerAddr)
	// t.ZeromqServerConf.host = "tcp://" + addr.Hostname() // this needs Go >=1.8
	t.ZeromqServerConf.host = "tcp://" + strings.Split(addr.Host, ":")[0]

	t.PublicKey, err = zeromq.ReadKeyFile(os.Getenv(EnvPublicKey), DefaultPublicKeyPath)
	if err != nil {
		return nil, fmt.Errorf("error reading file: %s", err)
	}

	// LOAD AND REPLACE WITH ENV VARIABLES
	var changed bool

	id := os.Getenv("ID")
	if id == "" && t.AutoGenID == "" {
		t.AutoGenID = uuid.NewV4().String()
		log.Println("Generated target ID:", t.AutoGenID)
		t.ID = t.AutoGenID
		changed = true
	} else if id == "" && t.ID != t.AutoGenID {
		log.Println("Taking previously generated ID:", t.AutoGenID)
		t.ID = t.AutoGenID
		changed = true
	} else if id != "" && id != t.ID {
		log.Println("Taking ID from env var:", id)
		t.ID = id
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
	if !reflect.DeepEqual(tags, t.Tags) {
		t.Tags = tags
		changed = true
	}

	latString := os.Getenv("LOCATION_LAT")
	lonString := os.Getenv("LOCATION_LON")
	if latString != "" && lonString != "" {
		lat, err := strconv.ParseFloat(latString, 64)
		if err != nil {
			return nil, fmt.Errorf("error parsing lat: %s", err)
		}
		lon, err := strconv.ParseFloat(lonString, 64)
		if err != nil {
			return nil, fmt.Errorf("error parsing lon: %s", err)
		}
		if t.Location == nil {
			t.Location = &model.Location{Lat: lat, Lon: lon}
			changed = true
		} else {
			if t.Location.Lat != lat {
				t.Location.Lat = lat
				changed = true
			}
			if t.Location.Lon != lon {
				t.Location.Lon = lon
				changed = true
			}
		}
	} else if t.Location != nil {
		t.Location = nil
		changed = true
	}

	if changed {
		t.saveState()
	}
	return t, nil
}

func loadState() (*target, error) {
	if _, err := os.Stat(DefaultStateFile); os.IsNotExist(err) {
		return nil, err
	}

	b, err := ioutil.ReadFile(DefaultStateFile)
	if err != nil {
		return nil, fmt.Errorf("error reading state file: %s", err)
	}

	var t target
	err = json.Unmarshal(b, &t)
	if err != nil {
		return nil, fmt.Errorf("error parsing state file: %s", err)
	}

	log.Println("Loaded state file:", DefaultStateFile)

	return &t, nil
}

func (t *target) saveState() {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	b, _ := json.MarshalIndent(t, "", "\t")
	err := ioutil.WriteFile(DefaultStateFile, b, 0600)
	if err != nil {
		log.Printf("Error saving state: %s", err)
		return
	}
	log.Println("Saved state:", DefaultStateFile)
}
