package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"code.linksmart.eu/dt/deployment-tool/manager/model"
	"code.linksmart.eu/dt/deployment-tool/manager/source"
	"code.linksmart.eu/dt/deployment-tool/manager/storage"
	"github.com/cskr/pubsub"
	uuid "github.com/satori/go.uuid"
)

type manager struct {
	storage storage.Storage
	pipe    model.Pipe
	events  *pubsub.PubSub
}

const (
	EventLogs          = "logs"
	EventTargetAdded   = "targetAdded"
	EventTargetUpdated = "targetUpdated"
)

type event struct {
	Topic   string      `json:"topic"`
	Payload interface{} `json:"payload"`
}

func startManager(pipe model.Pipe, storageDSN string) (*manager, error) {
	s, err := storage.NewElasticStorage(storageDSN)
	if err != nil {
		return nil, err
	}

	m := &manager{
		storage: s,
		pipe:    pipe,
		events:  pubsub.New(10),
	}

	go m.manageResponses()
	return m, nil
}

func (m *manager) addOrder(order *storage.Order) error {
	// add system generated meta values
	order.ID = m.newTaskID()
	order.Created = model.UnixTime()

	// cleanup

	if order.Build != nil && len(order.Build.Commands)+len(order.Build.Artifacts)+len(order.Build.Host) == 0 {
		order.Build = nil
	}
	if order.Deploy != nil && len(order.Deploy.Install.Commands)+len(order.Deploy.Run.Commands) == 0 {
		order.Deploy = nil
	}

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

	// check if deploy matches any targets
	if order.Deploy != nil {
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
	}

	// place into work directory
	err := m.fetchSource(order.ID, order.Source)
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

func (m *manager) getOrders(sortAsc bool, page, perPage int) ([]storage.Order, int64, error) {
	orders, total, err := m.storage.GetOrders(sortAsc, int((page-1)*perPage), perPage)
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

func (m *manager) stopOrder(id string) (found bool, err error) {
	order, err := m.storage.GetOrder(id)
	if err != nil {
		return false, fmt.Errorf("error querying order: %s", err)
	}
	if order == nil {
		return false, nil
	}
	var list []string
	if order.Deploy != nil {
		list = append(list, order.Deploy.Match.List...)
	}
	if order.Build != nil {
		var exist bool
		for i := range list {
			if order.Build.Host == list[i] {
				exist = true
			}
		}
		if !exist {
			list = append(list, order.Build.Host)
		}
	}
	for i := range list {
		m.requestStopAll(list[i])
	}
	return true, nil
}

func (m *manager) getTargets(tags []string, page, perPage int) ([]storage.Target, int64, error) {
	targets, total, err := m.storage.GetTargets(tags, int((page-1)*perPage), perPage)
	if err != nil {
		return nil, 0, fmt.Errorf("error querying targets: %s", err)
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

func (m *manager) getLogs(target, task, stage, command, output, error, sortField string, sortAsc bool, page, perPage int) ([]storage.Log, int64, error) {
	logs, total, err := m.storage.GetLogs(
		target,
		task,
		stage,
		command,
		output,
		error,
		sortField, sortAsc, int((page-1)*perPage), perPage)
	if err != nil {
		return nil, 0, fmt.Errorf("error querying logs: %s", err)
	}
	return logs, total, nil
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
	defer recovery()
	log.Println("processPackage", p.Task, p.Assembler, len(p.Payload))
	m.storeLog(p.Task, model.StageBuild, fmt.Sprintf("received package sized %d bytes", len(p.Payload)), false, p.Assembler)

	err := m.decompressPackage(p.Task, p.Payload)
	if err != nil {
		m.storeLogFatal(p.Task, model.StageBuild, "error decompressing assembled package", p.Assembler)
		return
	}

	order, err := m.storage.GetOrder(p.Task)
	if err != nil {
		m.storeLogFatal(p.Task, model.StageBuild, fmt.Sprintf("error querying order: %s", err), p.Assembler)
		return
	}

	if order.Deploy == nil {
		m.storeLogFatal(p.Task, model.StageBuild, fmt.Sprintf("no deployment instructions for package"), p.Assembler)
		log.Println("No deployment instructions for package.")
		return
	}

	m.storeLog(p.Task, model.StageBuild, model.StageEnd, false, p.Assembler)

	order.Build = nil
	m.composeTask(order)
}

func (m *manager) compressSource(orderID string) ([]byte, error) {
	if path := m.sourcePath(orderID); path != "" {
		compressedArchive, err := model.CompressFiles(path)
		if err != nil {
			return nil, err
		}
		return compressedArchive, nil
	}
	return nil, nil
}

func (m *manager) decompressPackage(orderID string, archive []byte) error {
	return model.DecompressFiles(archive, fmt.Sprintf("%s/%s/%s", source.OrdersDir, orderID, source.PackageDir))
}

func (m *manager) composeTask(order *storage.Order) {
	defer recovery()
	// a single order can result in two tasks: build and deploy
	if order.Build != nil {
		m.storeLog(order.ID, model.StageBuild, model.StageStart, false, order.Build.Host)

		compressedArchive, err := m.compressSource(order.ID)
		if err != nil {
			m.storeLogFatal(order.ID, model.StageBuild, fmt.Sprintf("error compressing files: %s", err), order.Build.Host)
			return
		}
		if len(compressedArchive) > 0 {
			m.storeLog(order.ID, model.StageBuild, fmt.Sprintf("compressed to %d bytes", len(compressedArchive)), false, order.Build.Host)
		}

		task := model.Task{
			Header:    order.Header,
			Build:     &order.Build.Build,
			Artifacts: compressedArchive,
		}
		err = task.Validate()
		if err != nil {
			m.storeLogFatal(order.ID, model.StageBuild, fmt.Sprintf("invalid task: %s", err), order.Build.Host)
			return
		}

		match := storage.Match{IDs: []string{order.Build.Host}, List: []string{order.Build.Host}} // just one device
		m.sendTask(&task, match)
	} else {
		m.storeLog(order.ID, model.StageInstall, model.StageStart, false, order.Deploy.Match.List...)

		compressedArchive, err := m.compressSource(order.ID)
		if err != nil {
			m.storeLogFatal(order.ID, model.StageInstall, fmt.Sprintf("error compressing files: %s", err), order.Deploy.Match.List...)
		}
		if len(compressedArchive) > 0 {
			m.storeLog(order.ID, model.StageInstall, fmt.Sprintf("compressed to %d bytes", len(compressedArchive)), false, order.Deploy.Match.List...)
		}

		task := model.Task{
			Header:    order.Header,
			Deploy:    &order.Deploy.Deploy,
			Artifacts: compressedArchive,
		}
		err = task.Validate()
		if err != nil {
			m.storeLogFatal(order.ID, model.StageInstall, fmt.Sprintf("invalid task: %s", err), order.Deploy.Match.List...)
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

	var stage string
	if task.Build != nil {
		ann.Type = model.TaskTypeBuild
		stage = model.StageBuild
	} else {
		ann.Type = model.TaskTypeDeploy
		stage = model.StageInstall
	}

	backOff := 0
	const maxAttempt = 3
	pending := make([]string, len(match.List))
	copy(pending, match.List)
	m.storeLog(task.ID, stage, "sending task", false, match.List...)

	for attempt := 1; attempt <= maxAttempt; attempt++ {
		//log.Printf("Sending task %s/%d to %s Attempt %d/%d", task.ID, ann.Type, receiverTopics, attempt, maxAttempt)

		// send announcement
		w := model.RequestWrapper{Announcement: &ann}
		b, _ := json.Marshal(w)
		for _, topic := range receiverTopics {
			m.pipe.RequestCh <- model.Message{topic, b}
		}
		//m.logTransfer(task.ID, "sent announcement", match.List...)

		time.Sleep(time.Second)

		// send actual task
		b, err := json.Marshal(&task)
		if err != nil {
			m.storeLogFatal(task.ID, stage, fmt.Sprintf("error serializing task: %s", err), match.List...)
			return
		}
		m.pipe.RequestCh <- model.Message{task.ID, b}
		//m.logTransfer(task.ID, "sent task", match.List...)

		// TODO resend when device is online?
		backOff += 10
		time.Sleep(time.Duration(backOff) * time.Second)

		var pendingTemp []string
		for _, target := range pending {
			// TODO this doesn't check which part of the task was delivered!!
			delivered, err := m.storage.DeliveredTask(target, task.ID)
			if err != nil {
				m.storeLogFatal(task.ID, stage, fmt.Sprintf("error searching for delivered task: %s", err), target)
				break
			}
			if !delivered {
				log.Printf("send attempt %d/%d: Unable to deliver %s/%d to %s", attempt, maxAttempt, task.ID, ann.Type, target)
				pendingTemp = append(pendingTemp, target)
			}
		}
		pending = pendingTemp

		if len(pending) == 0 {
			break
		}
		m.storeLog(task.ID, stage, fmt.Sprintf("not delivered. Attempt %d/%d", attempt, maxAttempt), false, pending...)
	}
	if len(pending) > 0 {
		m.storeLogFatal(task.ID, stage, "unable to deliver", pending...)
	}
	log.Printf("Task %s/%d received by %d/%d.", task.ID, ann.Type, len(match.List)-len(pending), len(match.List))
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

func (m *manager) requestStopAll(targetID string) {
	stopAll := true
	b, _ := json.Marshal(&model.RequestWrapper{StopAll: &stopAll})
	m.pipe.RequestCh <- model.Message{model.FormatTopicID(targetID), b}
}

func (m *manager) manageResponses() {
	defer recovery()
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
	}
}

func (m *manager) processTarget(target *storage.Target) {
	defer recovery()
	log.Println("Discovered target:", target.ID, target.Tags, target.Location)

	target.UpdatedAt = model.UnixTime()

	created, err := m.storage.AddTarget(target)
	if err != nil {
		log.Printf("Error adding target: %s", err)
		return
	}
	if created {
		m.publishEvent(EventTargetAdded, target)
		return
	}
	m.publishEvent(EventTargetUpdated, target)

}

func (m *manager) processResponse(response *model.Response) {
	defer recovery()
	log.Printf("Processing response from %s (len=%d)", response.TargetID, len(response.Logs))

	defer func(start time.Time) { log.Println("Processing response took", time.Since(start)) }(time.Now())

	// response to log request
	if response.OnRequest {
		target := storage.Target{
			LogRequestAt: response.Logs[len(response.Logs)-1].Time,
		}
		_, err := m.storage.PatchTarget(response.TargetID, &target)
		if err != nil {
			log.Printf("Error updating target: %s", err)
		}
	}

	// convert and store
	logs := make([]storage.Log, len(response.Logs))
	for i := range response.Logs {
		logs[i] = storage.Log{Log: response.Logs[i], Target: response.TargetID}
	}
	err := m.storage.AddLogs(logs)
	if err != nil {
		log.Printf("Error storing logs: %s", err)
		return
	}
	m.publishEvent(EventLogs, logs)
}

func (m *manager) storeLog(order, stage, message string, error bool, targets ...string) {
	logs := make([]storage.Log, len(targets))
	time := model.UnixTime()
	for i := range targets {
		logs[i] = storage.Log{Log: model.Log{
			Command: model.CommandByManager,
			Output:  message,
			Task:    order,
			Stage:   stage,
			Error:   error,
			Time:    time,
		}, Target: targets[i]}
	}
	err := m.storage.AddLogs(logs)
	if err != nil {
		log.Printf("Error storing logs: %s", err)
		return
	}
	m.publishEvent(EventLogs, logs)
}

func (m *manager) storeLogFatal(order, stage, message string, targets ...string) {
	log.Println("Fatal order error:", message)
	m.storeLog(order, stage, message, true, targets...)
	m.storeLog(order, stage, model.StageEnd, true, targets...)
}

func (m *manager) publishEvent(topic string, payload interface{}) {
	m.events.TryPub(event{Topic: topic, Payload: payload}, topic)
}
