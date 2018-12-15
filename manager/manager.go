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

const (
	maxTasksInMemory = 2
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

	m.targets = make(map[string]*target)
	m.orders = make(map[string]*order)

	go m.manageResponses()
	return m, nil
}

func (m *manager) addOrder(order *order) error {
	m.Lock()
	defer m.Unlock()

	var topics []string

TARGETS:
	for id, target := range m.targets {
		// target by id
		for _, id2 := range order.Target.IDs {
			if id == id2 {
				topics = append(topics, model.FormatTopicID(id))
				order.Receivers = append(order.Receivers, id)
				continue TARGETS
			}
		}
		// target by tag
		for _, t := range target.Tags {
			for _, t2 := range order.Target.Tags {
				if t == t2 {
					topics = append(topics, model.FormatTopicTag(t))
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
		go m.sendTask(&ann, &task, topics, order.Receivers)
	}

	return nil
}

func (m *manager) getOrders() (map[string]*order, error) {
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

func (m *manager) sendTask(ann *model.Announcement, task *model.Task, topics, matchingTargets []string) {

	for pending := true; pending; {
		log.Printf("Sending task %s to %s", task.ID, topics)
		//log.Printf("sendTask: %+v", task)

		// send announcement
		w := model.RequestWrapper{Announcement: ann}
		b, _ := json.Marshal(w)
		for _, topic := range topics {
			m.pipe.RequestCh <- model.Message{topic, b}
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
			if _, found := m.targets[match].Logs[task.ID]; !found {
				pending = true
			}
		}
	}
	log.Println("Task received by all targets.")
}

func (m *manager) requestLogs(targetID string) error {
	w := model.RequestWrapper{LogRequest: &model.LogRequest{
		IfModifiedSince: m.targets[targetID].LastLogRequest,
	}}
	b, _ := json.Marshal(&w)
	m.pipe.RequestCh <- model.Message{
		Topic:   model.FormatTopicID(targetID),
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

		m.targets[response.TargetID].Logs[l.Task].insert(l)
	}
	// remove old tasks
	if len(m.targets[response.TargetID].Logs) > maxTasksInMemory {
		var times []int64
		for k := range m.targets[response.TargetID].Logs {
			// TODO this should not be needed if registry is persisted
			if _, found := m.orders[k]; !found {
				log.Println("Adding missing order to registry (Created=0):", k)
				m.orders[k] = &order{}
				m.orders[k].ID = k
				m.orders[k].Created = 0
			}
			times = append(times, m.orders[k].Created)
		}
		sort.Slice(times, func(i, j int) bool { return times[i] < times[j] })
		// delete the oldest item(s)
		pivot := times[len(times)-maxTasksInMemory]
		for k := range m.targets[response.TargetID].Logs {
			if m.orders[k].Created == 0 || m.orders[k].Created < pivot {
				log.Println("Removing logs for", k)
				delete(m.targets[response.TargetID].Logs, k)
			}
		}
	}
	log.Println("Processing response took", time.Since(start))
}
