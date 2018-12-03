package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"code.linksmart.eu/dt/deployment-tool/manager/model"
	"github.com/mholt/archiver"
)

type manager struct {
	sync.RWMutex
	registry

	pipe   model.Pipe
	update *sync.Cond
}

func startManager(pipe model.Pipe) (*manager, error) {
	m := &manager{
		pipe:   pipe,
		update: sync.NewCond(&sync.Mutex{}),
	}

	m.targets = make(map[string]*Target)
	m.orders = make(map[string]*Order)

	go m.manageResponses()
	return m, nil
}

func (m *manager) addOrder(order *Order) error {
	m.Lock()
	defer m.Unlock()

TARGETS:
	for id, target := range m.targets {
		for _, t := range target.Tags {
			for _, t2 := range order.Target.Tags {
				if t == t2 {
					order.Receivers = append(order.Receivers, id)
					continue TARGETS
				}
			}
		}
	}
	log.Println("Order receivers:", len(order.Receivers))

	var compressedArchive []byte
	var err error
	if len(order.Stages.Transfer) > 0 {
		compressedArchive, err = m.compressFiles(order.Stages.Transfer)
		if err != nil {
			return fmt.Errorf("error compressing files: %s", err)
		}
	}

	ann := model.Announcement{
		Header: order.Header,
		Size:   len(compressedArchive),
	}

	task := model.Task{
		Header:    order.Header,
		Stages:    order.Stages,
		Artifacts: compressedArchive,
	}

	m.orders[order.ID] = order
	log.Println("Added order:", order.ID)

	if len(order.Receivers) > 0 {
		go m.sendTask(&ann, &task, order.Target.Tags, order.Receivers)
	}

	return nil
}

func (m *manager) getOrders() (map[string]*Order, error) {
	m.RLock()
	defer m.RUnlock()
	return m.orders, nil
}

func (m *manager) compressFiles(filePaths []string) ([]byte, error) {
	var b bytes.Buffer
	err := archiver.TarGz.Write(&b, filePaths)
	if err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func (m *manager) sendTask(ann *model.Announcement, task *model.Task, targetTags, matchingTargets []string) {

	for pending := true; pending; {
		log.Printf("sendTask: %s", task.ID)
		//log.Printf("sendTask: %+v", task)

		// send announcement
		b, _ := json.Marshal(ann)
		for _, tag := range targetTags {
			m.pipe.RequestCh <- model.Message{model.TargetTag(tag), b}
		}

		time.Sleep(time.Second)

		// send actual task
		b, err := json.Marshal(&task)
		if err != nil {
			log.Printf("Error serializing task: %s", err)
		}
		m.pipe.RequestCh <- model.Message{task.ID, b}

		time.Sleep(10 * time.Second)

		// TODO which messages are received, what is pending?
		pending = false
		for _, match := range matchingTargets {
			if _, found := m.targets[match].Tasks[task.ID]; !found {
				pending = true
			}
		}
	}
	log.Println("Task received by all targets.")
}

func (m *manager) requestLogs(targetID string) error {
	b, _ := json.Marshal(&model.LogRequest{m.targets[targetID].LastLogRequest})
	m.pipe.RequestCh <- model.Message{
		Topic:   model.TargetTopic(targetID),
		Payload: b,
	}
	return nil
}

func (m *manager) manageResponses() {
	for resp := range m.pipe.ResponseCh {
		switch resp.Topic {
		case string(model.ResponseAdvertisement):
			var target model.Target
			err := json.Unmarshal(resp.Payload, &target)
			if err != nil {
				log.Printf("error parsing response: %s", err)
				log.Printf("payload was: %s", string(resp.Payload))
				continue
			}
			m.processTarget(&target)

		default:
			var response model.Response
			err := json.Unmarshal(resp.Payload, &response)
			if err != nil {
				log.Printf("error parsing response: %s", err)
				log.Printf("payload was: %s", string(resp.Payload))
				continue
			}
			m.processResponse(&response)
		}
		// sent update notification
		m.update.Broadcast()
	}
}

func (m *manager) processTarget(target *model.Target) {
	log.Printf("Discovered target: %s %v %s", target.ID, target.Tags, target.TaskID)

	m.Lock()
	defer m.Unlock()

	if _, found := m.targets[target.ID]; !found {
		m.targets[target.ID] = newTarget()
	}
	m.targets[target.ID].Tags = target.Tags
}

func (m *manager) processResponse(response *model.Response) {
	log.Println("Processing response", response)

	m.Lock()
	defer m.Unlock()
	start := time.Now()

	if _, found := m.targets[response.TargetID]; !found {
		log.Println("Log from unknown target:", response.TargetID)
		return
	}

	// response to log request
	if response.OnRequest {
		sync := response.Logs[len(response.Logs)-1].Time
		m.targets[response.TargetID].LastLogRequest = sync
	}

	for _, l := range response.Logs {
		m.targets[response.TargetID].initTask(l.Task)

		// create aliases
		task := m.targets[response.TargetID].Tasks[l.Task]
		stageLogs := task.GetStageLog(l.Stage)
		// update task
		task.Updated = model.UnixTime()
		stageLogs.InsertLogs(l)
	}
	// remove old tasks
	const max = 2
	if len(m.targets[response.TargetID].Tasks) > max {
		var times []int64
		for k := range m.targets[response.TargetID].Tasks {
			times = append(times, m.orders[k].Created)
		}
		sort.Slice(times, func(i, j int) bool { return times[i] < times[j] })
		// delete the oldest item(s)
		pivot := times[len(times)-max]
		for k := range m.targets[response.TargetID].Tasks {
			if m.orders[k].Created < pivot {
				log.Println("Removing logs for", k)
				delete(m.targets[response.TargetID].Tasks, k)
			}
		}
	}
	log.Println("Processing response took", time.Since(start))
}
