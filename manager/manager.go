package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"code.linksmart.eu/dt/deployment-tool/manager/model"
	"code.linksmart.eu/dt/deployment-tool/manager/source"
	"code.linksmart.eu/dt/deployment-tool/manager/storage"
	uuid "github.com/satori/go.uuid"
)

type manager struct {
	storage storage.Storage
	pipe    model.Pipe
	update  *sync.Cond
}

func startManager(pipe model.Pipe) (*manager, error) {
	storage, err := storage.NewElasticStorage("http://127.0.0.1:9200")
	if err != nil {
		return nil, err
	}

	m := &manager{
		storage: storage,
		pipe:    pipe,
		update:  sync.NewCond(&sync.Mutex{}),
	}

	go m.manageResponses()
	return m, nil
}

func (m *manager) addOrder(order *storage.Order) error {
	// add system generated meta values
	order.ID = m.newTaskID()
	order.Created = model.UnixTime()

	// check if build host exists
	if order.Build != nil {
		target, err := m.storage.GetTarget(order.Build.Host)
		if err != nil {
			return fmt.Errorf("error getting build host: %s", err)
		}
		if target == nil {
			return fmt.Errorf("build host not found: %s", order.Build.Host)
		}
	}

	receivers, hitIDs, hitTags, err := m.storage.MatchTargets(order.Deploy.Target.IDs, order.Deploy.Target.Tags)
	if err != nil {
		return fmt.Errorf("error matching targets: %s", err)

	}
	if len(receivers) == 0 {
		return fmt.Errorf("deployment matches no targets")
	}
	order.Deploy.Match.IDs = hitIDs
	order.Deploy.Match.Tags = hitTags
	order.Deploy.Match.List = receivers

	// place into work directory
	err = m.fetchSource(order.ID, order.Source)
	if err != nil {
		return fmt.Errorf("error fetching source files: %s", err)
	}

	order.Source = nil
	err = m.storage.AddOrder(order)
	if err != nil {
		return fmt.Errorf("error storing order: %s", err)
	}
	log.Println("Added order:", order.ID)
	// sent update notification
	//m.update.Broadcast() // TODO this only sends targets

	go m.composeTask(order)
	return nil
}

func (m *manager) targetTopics(ids, tags []string) []string {
	var receiverTopics []string
	for _, id := range ids {
		receiverTopics = append(receiverTopics, model.FormatTopicID(id))
	}
	for _, tag := range tags {
		receiverTopics = append(receiverTopics, model.FormatTopicTag(tag))
	}
	return receiverTopics
}

func (m *manager) newTaskID() string {
	return uuid.NewV4().String()
}

func (m *manager) getOrders() ([]storage.Order, int64, error) {
	orders, total, err := m.storage.GetOrders()
	if err != nil {
		return nil, 0, fmt.Errorf("error querying orders: %s", err)
	}
	return orders, total, nil
}

func (m *manager) getOrder(id string) (*storage.Order, error) {
	order, err := m.storage.GetOrder(id)
	if err != nil {
		return nil, fmt.Errorf("error querying order: %s", err)
	}
	return order, nil
}

func (m *manager) getTargets() ([]storage.Target, int64, error) {
	targets, total, err := m.storage.GetTargets([]string{}, []string{"amd64", "swarm"}, 0, 100) // TODO pass pagination
	if err != nil {
		log.Println(err)
	}
	return targets, total, nil
}

func (m *manager) getTarget(id string) (*storage.Target, error) {
	target, err := m.storage.GetTarget(id)
	if err != nil {
		return nil, fmt.Errorf("error querying target: %s", err)
	}
	return target, nil
}

func (m *manager) searchLogs(search map[string]interface{}) ([]storage.Log, int64, error) {
	logs, total, err := m.storage.SearchLogs(search)
	if err != nil {
		return nil, 0, fmt.Errorf("error querying logs: %s", err)
	}
	return logs, total, nil
}

func (m *manager) fetchSource(orderID string, src *source.Source) error {
	switch {
	case src.Paths != nil:
		return src.Paths.Copy(orderID)
	case src.Zip != nil:
		return src.Zip.Store(orderID)
		//case src.Git != nil:
		//	return src.Git.Clone(orderID)
		//case src.Order != nil:
		//	return src.Order.Fetch(orderID)
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

	order, err := m.storage.GetOrder(p.Task)
	if err != nil {
		m.logTransferFatal(p.Task, fmt.Sprintf("error querying order: %s", err), p.Assembler)
		return
	}

	order.Build = nil
	m.composeTask(order)
}

func (m *manager) logTransfer(order, message string, targets ...string) {
	for _, target := range targets {
		err := m.storage.AddLog(&storage.Log{Log: model.Log{
			Command: model.CommandByManager,
			Output:  message,
			Task:    order,
			Stage:   model.StageTransfer,
			Time:    model.UnixTime(),
		}, Target: target})
		if err != nil {
			log.Printf("Error storing log_: %s", err)
		}
	}
	// sent update notification
	m.update.Broadcast()
}

func (m *manager) logTransferFatal(order, message string, targets ...string) {
	log.Println("Fatal error:", message)
	for _, target := range targets {
		// the error message
		err := m.storage.AddLog(&storage.Log{Log: model.Log{
			Command: model.CommandByManager,
			Output:  message,
			Task:    order,
			Stage:   model.StageTransfer,
			Time:    model.UnixTime(),
			Error:   true,
		}, Target: target})
		if err != nil {
			log.Printf("Error storing log: %s", err)
		}
		// end flag
		err = m.storage.AddLog(&storage.Log{Log: model.Log{
			Command: model.CommandByManager,
			Output:  model.StageEnd,
			Task:    order,
			Stage:   model.StageTransfer,
			Time:    model.UnixTime(),
			Error:   true,
		}, Target: target})
		if err != nil {
			log.Printf("Error storing log_: %s", err)
		}
	}
	// sent update notification
	m.update.Broadcast()
}

func (m *manager) compressSource(orderID string, receivers ...string) ([]byte, error) {
	m.logTransfer(orderID, model.StageStart, receivers...)
	if path := m.sourcePath(orderID); path != "" {
		compressedArchive, err := model.CompressFiles(path)
		if err != nil {
			return nil, err
		}
		m.logTransfer(orderID, fmt.Sprintf("compressed to %d bytes", len(compressedArchive)), receivers...)
		return compressedArchive, nil
	}
	m.logTransfer(orderID, "no source files to transfer", receivers...)
	return nil, nil
}

func (m *manager) composeTask(order *storage.Order) {
	// a single order can result in two tasks: build and deploy

	if order.Build != nil {
		compressedArchive, err := m.compressSource(order.ID, order.Build.Host)
		if err != nil {
			m.logTransferFatal(order.ID, fmt.Sprintf("error compressing files: %s", err), order.Build.Host)
			return
		}

		task := model.Task{
			Header:    order.Header,
			Build:     &order.Build.Build,
			Artifacts: compressedArchive,
		}
		err = task.Validate()
		if err != nil {
			m.logTransferFatal(order.ID, fmt.Sprintf("invalid task: %s", err), order.Build.Host)
			return
		}

		match := storage.Match{IDs: []string{order.Build.Host}, List: []string{order.Build.Host}} // just one device
		m.sendTask(&task, match)
	} else {

		compressedArchive, err := m.compressSource(order.ID, order.Deploy.Match.List...)
		if err != nil {
			m.logTransferFatal(order.ID, fmt.Sprintf("error compressing files: %s", err), order.Deploy.Match.List...)
		}

		task := model.Task{
			Header:    order.Header,
			Deploy:    &order.Deploy.Deploy,
			Artifacts: compressedArchive,
		}
		err = task.Validate()
		if err != nil {
			m.logTransferFatal(order.ID, fmt.Sprintf("invalid task: %s", err), order.Deploy.Match.List...)
			return
		}

		m.sendTask(&task, order.Deploy.Match)
	}

}

func (m *manager) sendTask(task *model.Task, match storage.Match) {

	receiverTopics := m.targetTopics(match.IDs, match.Tags)

	ann := model.Announcement{
		Header: task.Header,
		Size:   len(task.Artifacts),
	}

	if task.Build != nil {
		ann.Type = model.TaskTypeBuild
	} else {
		ann.Type = model.TaskTypeDeploy
	}

	for pending := true; pending; {
		log.Printf("Sending task %s/%d to %s", task.ID, ann.Type, receiverTopics)

		// send announcement
		w := model.RequestWrapper{Announcement: &ann}
		b, _ := json.Marshal(w)
		for _, topic := range receiverTopics {
			m.pipe.RequestCh <- model.Message{topic, b}
		}
		m.logTransfer(task.ID, "sent announcement", match.List...)

		time.Sleep(time.Second)

		// send actual task
		b, err := json.Marshal(&task)
		if err != nil {
			m.logTransferFatal(task.ID, fmt.Sprintf("error serializing task: %s", err), match.List...)
			return
		}
		m.pipe.RequestCh <- model.Message{task.ID, b}
		m.logTransfer(task.ID, "sent task", match.List...)

		time.Sleep(10 * time.Second)

		pending = false
		for _, target := range match.List {
			delivered, err := m.storage.DeliveredTask(target, task.ID)
			if err != nil {
				log.Printf("Error searching for delivered task: %s", err)
				m.logTransferFatal(task.ID, fmt.Sprintf("error searching for delivered task: %s", err), target)
				break
			}
			if delivered {
				log.Printf("Task %s/%d delivered to %s", task.ID, ann.Type, target)
				m.logTransfer(task.ID, model.StageEnd, target) // TODO send this as soon as a log message is received?
			} else {
				pending = true
				break
			}
		}
	}
	log.Printf("Task %s/%d received by all.", task.ID, ann.Type)
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
	target, err := m.storage.GetTarget(targetID)
	if err != nil {
		return fmt.Errorf("error querying target: %s", err)
	}
	w := model.RequestWrapper{LogRequest: &model.LogRequest{
		IfModifiedSince: target.LogRequestAt,
	}}
	b, _ := json.Marshal(&w)
	m.pipe.RequestCh <- model.Message{model.FormatTopicID(targetID), b}
	return nil
}

func (m *manager) manageResponses() {
	for resp := range m.pipe.ResponseCh {
		switch resp.Topic {
		case model.ResponseAdvertisement:
			var target storage.Target
			err := json.Unmarshal(resp.Payload, &target)
			if err != nil {
				log.Printf("error parsing advert response: %s", err)
				log.Printf("payload was: %s", string(resp.Payload))
				continue
			}
			go m.processTarget(&target)
		case model.ResponsePackage:
			var pkg model.Package
			err := json.Unmarshal(resp.Payload, &pkg)
			if err != nil {
				log.Printf("error parsing package response: %s", err)
				log.Printf("payload was: %s", string(resp.Payload))
				continue
			}
			go m.processPackage(&pkg)
		default:
			var response model.Response
			err := json.Unmarshal(resp.Payload, &response)
			if err != nil {
				log.Printf("error parsing response: %s", err)
				log.Printf("payload was: %s", string(resp.Payload))
				continue
			}
			go m.processResponse(&response)
		}
		// sent update notification
		m.update.Broadcast()
	}
}

func (m *manager) processTarget(target *storage.Target) {
	log.Printf("Discovered target: %s: %v", target.ID, target.Tags)

	// TODO update every time?
	target.UpdatedAt = model.UnixTime()
	err := m.storage.AddTarget(target)
	if err != nil {
		log.Printf("Error storing target: %s", err)
		return
	}
}

func (m *manager) processResponse(response *model.Response) {
	log.Printf("Processing response from %s (len=%d)", response.TargetID, len(response.Logs))

	start := time.Now()

	// response to log request
	if response.OnRequest {
		// TODO replace with
		fields := map[string]interface{}{
			"logRequestAt": response.Logs[len(response.Logs)-1].Time,
		}
		err := m.storage.PatchTarget(response.TargetID, fields)
		if err != nil {
			log.Printf("Error updating target: %s", err)
		}
	}

	for _, l := range response.Logs {
		// TODO send in bulk
		// https://www.elastic.co/guide/en/elasticsearch/reference/current/docs-bulk.html
		//l.Target = response.TargetID
		err := m.storage.AddLog(&storage.Log{Log: l, Target: response.TargetID})
		if err != nil {
			log.Printf("Error storing log: %s", err)
		}
	}

	log.Println("Processing response took", time.Since(start))
}
