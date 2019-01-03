package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"
	"sync"
	"time"

	"code.linksmart.eu/dt/deployment-tool/manager/model"
	"code.linksmart.eu/dt/deployment-tool/manager/source"
	"github.com/satori/go.uuid"
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
	// add system generated meta values
	order.ID = m.newTaskID()
	order.Created = time.Now().UnixNano()

	m.Lock()
	defer m.Unlock()

TARGETS:
	for id, target := range m.targets {
		// target by id
		for _, id2 := range order.Target.IDs {
			if id == id2 {
				order.receiverTopics = append(order.receiverTopics, model.FormatTopicID(id))
				order.Receivers = append(order.Receivers, id)
				continue TARGETS
			}
		}
		// target by tag
		for _, t := range target.Tags {
			for _, t2 := range order.Target.Tags {
				if t == t2 {
					order.receiverTopics = append(order.receiverTopics, model.FormatTopicTag(t))
					order.Receivers = append(order.Receivers, id)
					continue TARGETS
				}
			}
		}
	}

	if order.Target.Assembler != "" {
		if _, found := m.targets[order.Target.Assembler]; !found {
			return fmt.Errorf("target for assember is not found: %s", order.Target.Assembler)
		}
	}

	log.Println("Order receivers:", len(order.Receivers))
	if len(order.Receivers) == 0 {
		return fmt.Errorf("could not match any device")
	}

	// place into work directory
	err := m.fetchSource(order.ID, order.Source)
	if err != nil {
		return fmt.Errorf("error fetching source files: %s", err)
	}

	m.orders[order.ID] = order
	log.Println("Added order:", order.ID)

	go m.sendTask(order)
	return nil
}

func (m *manager) newTaskID() string {
	return uuid.NewV4().String()
}

func (m *manager) getOrders() (map[string]*order, error) {
	m.RLock()
	defer m.RUnlock()
	return m.orders, nil
}

func (m *manager) initLogger(orderID string, targetIDs ...string) {
	for _, targetID := range targetIDs {
		m.targets[targetID].initTask(orderID)
	}
}

func (m *manager) fetchSource(orderID string, src source.Source) error {
	switch {
	case src.Paths != nil:
		return src.Paths.Copy(orderID)
	case src.Zip != nil:
		return src.Zip.Store(orderID)
	case src.Git != nil:
		return src.Git.Clone(orderID)
	case src.Order != nil:
		return src.Order.Fetch(orderID)
	}
	return nil
}

func (m *manager) processPackage(p *model.Package) {
	/* TODO
	- send it to the assembler device
	- get back logs as usuall
	- get the final result as tar.gz
	- send acknowledgement and remove it on assembler
	- continue with the order
	*/
	log.Println("processPackage", p.Task, p.Assembler, len(p.Payload))

	err := model.DecompressFiles(p.Payload, fmt.Sprintf("%s/%s/%s", source.OrdersDir, p.Task, source.PackageDir))
	if err != nil {
		log.Printf("Error decompressing archive: %s", err) // TODO send to manager
		return
	}

	m.RLock()
	defer m.RUnlock()
	order, found := m.orders[p.Task]
	if !found {
		log.Printf("Package for unknown order: %s", p.Task)
		return
	}
	// assemble is done, make a new order for install and run
	go m.addOrder(order.getChild())
}

func (m *manager) insertLog(order, stage, message string, targets ...string) {
	for _, target := range targets {
		m.targets[target].Logs[order].insert(model.Log{
			Output: message,
			Task:   order,
			Stage:  stage,
			Time:   model.UnixTime(),
		})
	}
	// sent update notification
	m.update.Broadcast()
}

func (m *manager) insertLogError(order, stage, message string, targets ...string) {
	for _, target := range targets {
		m.targets[target].Logs[order].insert(model.Log{
			Output: message,
			Task:   order,
			Stage:  stage,
			Time:   model.UnixTime(),
			Error:  true,
		})
	}
	// sent update notification
	m.update.Broadcast()
}

func (m *manager) sendTask(order *order) {

	var (
		// instantiated based on the task type
		stages         model.Stages
		receivers      []string
		receiverTopics []string
	)
	if len(order.Stages.Assemble) != 0 {
		stages.Assemble = order.Stages.Assemble
		stages.Transfer = order.Stages.Transfer
		receivers = []string{order.Target.Assembler}
		receiverTopics = []string{model.FormatTopicID(order.Target.Assembler)}

		m.Lock()
		m.initLogger(order.ID, order.Target.Assembler)
		m.Unlock()
	} else {
		stages.Install = order.Stages.Install
		stages.Run = order.Stages.Run
		receivers = order.Receivers
		receiverTopics = order.receiverTopics

		m.Lock()
		m.initLogger(order.ID, order.Receivers...)
		m.Unlock()
	}

	var compressedArchive []byte
	m.insertLog(order.ID, model.StageTransfer, model.StageStart, receivers...)
	if path := m.sourcePath(order.ID); path != "" {
		var err error
		compressedArchive, err = model.CompressFiles(path)
		if err != nil {
			m.insertLogError(order.ID, model.StageTransfer, fmt.Sprintf("error compressing files: %s", err), receivers...)
			m.insertLogError(order.ID, model.StageTransfer, model.StageEnd, receivers...)

			log.Printf("error compressing files: %s", err)
			return
		}
		m.insertLog(order.ID, model.StageTransfer, fmt.Sprintf("compressed to %d bytes", len(compressedArchive)), receivers...)
	} else {
		m.insertLog(order.ID, model.StageTransfer, "no source files to transfer", receivers...)
	}

	ann := model.Announcement{
		Header: order.Header,
		Size:   len(compressedArchive),
	}

	task := model.Task{
		Header:    order.Header,
		Stages:    stages,
		Artifacts: compressedArchive,
	}

	for pending := true; pending; {
		log.Printf("Sending task %s to %s", task.ID, receiverTopics)
		//log.Printf("sendTask: %+v", task)

		// send announcement
		w := model.RequestWrapper{Announcement: &ann}
		b, _ := json.Marshal(w)
		for _, topic := range receiverTopics {
			m.pipe.RequestCh <- model.Message{topic, b}
		}
		m.insertLog(order.ID, model.StageTransfer, "sent announcement", receivers...)

		time.Sleep(time.Second)

		// send actual task
		b, err := json.Marshal(&task)
		if err != nil {
			log.Printf("Error serializing task: %s", err)
			// TODO add logs and abort?
			return
		}
		m.pipe.RequestCh <- model.Message{task.ID, b}
		m.insertLog(order.ID, model.StageTransfer, "sent task", receivers...)

		time.Sleep(10 * time.Second)

		// TODO which messages are received, what is pending?
		pending = false
		for _, match := range receivers {
			if log, found := m.targets[match].Logs[task.ID]; found {
				if len(log.Install)+len(log.Run) == 0 {
					pending = true
				} else {
					m.insertLog(order.ID, model.StageTransfer, model.StageEnd, match) // TODO add this as soon as an ack arrives
				}
			}
		}
	}
	log.Println("Task received by all targets.")
	// TODO
	// remove the directory
}

func (m *manager) sourcePath(orderID string) string {
	wd := fmt.Sprintf("%s/%s", source.OrdersDir, orderID)
	path := fmt.Sprintf("%s/%s/%s", source.OrdersDir, orderID, source.ExecDir(wd))

	if _, err := os.Stat(path); err != nil && os.IsNotExist(err) {
		return ""
	}
	return path
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
		case model.ResponseAdvertisement:
			var target model.Target
			err := json.Unmarshal(resp.Payload, &target)
			if err != nil {
				log.Printf("error parsing advert response: %s", err)
				log.Printf("payload was: %s", string(resp.Payload))
				continue
			}
			m.processTarget(&target)
		case model.ResponsePackage:
			var pkg model.Package
			err := json.Unmarshal(resp.Payload, &pkg)
			if err != nil {
				log.Printf("error parsing package response: %s", err)
				log.Printf("payload was: %s", string(resp.Payload))
				continue
			}
			m.processPackage(&pkg)
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
		m.initLogger(target.TaskID, target.ID)
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
		m.targets[response.TargetID].LastLogRequest = response.Logs[len(response.Logs)-1].Time
	}

	for _, l := range response.Logs {
		m.initLogger(l.Task, response.TargetID) // TODO let the device send all task ids in advertisement and init during discovery?
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
