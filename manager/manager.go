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

	m.RLock()
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
	m.RUnlock()

	if order.Target.Assembler != "" {
		m.RLock()
		if _, found := m.targets[order.Target.Assembler]; !found {
			m.RUnlock()
			return fmt.Errorf("target for assember is not found: %s", order.Target.Assembler)
		}
		m.RUnlock()
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

	m.Lock()
	m.orders[order.ID] = order
	m.Unlock()
	log.Println("Added order:", order.ID)
	// sent update notification
	m.update.Broadcast()

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

func (m *manager) getTargets() (map[string]*target, error) {
	m.RLock()
	defer m.RUnlock()
	return m.targets, nil
}

func (m *manager) getTarget(id string) (*target, error) {
	m.RLock()
	defer m.RUnlock()
	if _, found := m.targets[id]; !found {
		return nil, nil
	}
	return m.targets[id], nil
}

// requires lock
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
	m.logTransfer(p.Task, fmt.Sprintf("received package sized %d bytes", len(p.Payload)), p.Assembler)

	err := model.DecompressFiles(p.Payload, fmt.Sprintf("%s/%s/%s", source.OrdersDir, p.Task, source.PackageDir))
	if err != nil {
		m.logTransferFatal(p.Task, "error decompressing assembled package", p.Assembler)
		return
	}

	m.RLock()
	order, found := m.orders[p.Task]
	if !found {
		log.Printf("Package for unknown order: %s", p.Task)
		m.RUnlock()
		return
	}
	m.RUnlock()

	// assemble is done, make a new order for install and run
	child := order.getChild()
	err = m.addOrder(child)
	if err != nil {
		m.logTransferFatal(p.Task, fmt.Sprintf("error creating child order: %s", err), p.Assembler)
		return
	}
	m.logTransfer(p.Task, fmt.Sprintf("created child order: %s", child.ID), p.Assembler)

	m.Lock()
	order.ChildOrder = child.ID
	m.Unlock()
}

func (m *manager) logTransfer(order, message string, targets ...string) {
	m.RLock()
	for _, target := range targets {
		m.targets[target].Logs[order].insert(model.Log{
			Output: message,
			Task:   order,
			Stage:  model.StageTransfer,
			Time:   model.UnixTime(),
		})
	}
	m.RUnlock()
	// sent update notification
	m.update.Broadcast()
}

func (m *manager) logTransferFatal(order, message string, targets ...string) {
	m.RLock()
	for _, target := range targets {
		log.Println(message)
		m.targets[target].Logs[order].insert(model.Log{
			Output: message,
			Task:   order,
			Stage:  model.StageTransfer,
			Time:   model.UnixTime(),
			Error:  true,
		})
		m.targets[target].Logs[order].insert(model.Log{
			Output: model.StageEnd,
			Task:   order,
			Stage:  model.StageTransfer,
			Time:   model.UnixTime(),
			Error:  true,
		})
	}
	m.RUnlock()
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
	m.logTransfer(order.ID, model.StageStart, receivers...)
	if path := m.sourcePath(order.ID); path != "" {
		var err error
		compressedArchive, err = model.CompressFiles(path)
		if err != nil {
			m.logTransferFatal(order.ID, fmt.Sprintf("error compressing files: %s", err), receivers...)
			return
		}
		m.logTransfer(order.ID, fmt.Sprintf("compressed to %d bytes", len(compressedArchive)), receivers...)
	} else {
		m.logTransfer(order.ID, "no source files to transfer", receivers...)
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
		m.logTransfer(order.ID, "sent announcement", receivers...)

		time.Sleep(time.Second)

		// send actual task
		b, err := json.Marshal(&task)
		if err != nil {
			m.logTransferFatal(order.ID, fmt.Sprintf("error serializing task: %s", err), receivers...)
			return
		}
		m.pipe.RequestCh <- model.Message{task.ID, b}
		m.logTransfer(order.ID, "sent task", receivers...)

		time.Sleep(10 * time.Second)

		// TODO which messages are received, what is pending?
		pending = false
		m.RLock()
		for _, match := range receivers {
			if l, found := m.targets[match].Logs[task.ID]; found {
				if len(l.Install)+len(l.Run) == 0 {
					pending = true
				} else {
					m.logTransfer(order.ID, model.StageEnd, match) // TODO add this as soon as an ack arrives
				}
			}
		}
		m.RUnlock()
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
	m.RLock()
	w := model.RequestWrapper{LogRequest: &model.LogRequest{
		IfModifiedSince: m.targets[targetID].LastLogRequest,
	}}
	m.RUnlock()
	b, _ := json.Marshal(&w)
	m.pipe.RequestCh <- model.Message{model.FormatTopicID(targetID), b}
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
