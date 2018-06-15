package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"code.linksmart.eu/dt/deployment-tool/model"
	zmq "github.com/pebbe/zmq4"
)

const (
	// <arch>.<os>.<distro>.<os_version>.<hw>.<hw_version>
	ackTopic = "ACK-"
	resTopic = "RES-"
)

type zmqClient struct {
	subscriber *zmq.Socket
	publisher  *zmq.Socket
	pubMonitor *zmq.Socket

	pipe model.Pipe
}

func startZMQClient(subEndpoint, pubEndpoint, agentID string, pipe model.Pipe) (*zmqClient, error) {
	log.Printf("Using ZeroMQ v%v", strings.Replace(fmt.Sprint(zmq.Version()), " ", ".", -1))
	log.Println("Sub endpoint:", subEndpoint)
	log.Println("Pub endpoint:", pubEndpoint)

	c := &zmqClient{
		pipe: pipe,
	}

	var err error
	// socket to receive from server
	c.subscriber, err = zmq.NewSocket(zmq.SUB)
	if err != nil {
		return nil, fmt.Errorf("error creating SUB socket: %s", err)
	}
	err = c.subscriber.Connect(subEndpoint)
	if err != nil {
		return nil, fmt.Errorf("error connecting to SUB endpoint: %s", err)
	}
	// socket to send to server
	c.publisher, err = zmq.NewSocket(zmq.PUB)
	if err != nil {
		return nil, fmt.Errorf("error creating PUB socket: %s", err)
	}
	err = c.publisher.Connect(pubEndpoint)
	if err != nil {
		return nil, fmt.Errorf("error connecting to PUB endpoint: %s", err)
	}

	c.monitor()
	go c.startListener()
	go c.startResponder()

	// subscribe to fixed topics
	err = c.subscriber.SetSubscribe(model.RequestTaskAnnouncement)
	if err != nil {
		return nil, fmt.Errorf("error subscribing: %s", err)
	}
	err = c.subscriber.SetSubscribe(agentID)
	if err != nil {
		return nil, fmt.Errorf("error subscribing: %s", err)
	}

	return c, nil
}

func (c *zmqClient) startListener() {
	for {
		msg, err := c.subscriber.Recv(0)
		if err != nil {
			log.Println("ERROR:", err) // TODO send to manager
			continue
		}
		// split the prefix
		parts := strings.SplitN(msg, ":", 2)
		if len(parts) != 2 {
			log.Fatalln("Unable to parse message") // TODO send to manager
			continue
		}
		// send to worker
		c.pipe.RequestCh <- model.Request{parts[0], []byte(parts[1])}
	}
}

func (c *zmqClient) startResponder() {
	for resp := range c.pipe.ResponseCh {
		// set response publish topic
		topic := string(model.ResponseUnspecified)

		// on-demand subscription
		if resp.ResponseType == model.ResponseAck {
			err := c.subscriber.SetSubscribe(resp.TaskID)
			if err != nil {
				log.Println(err)
			}
			log.Println("Subscribed to task", resp.TaskID)
		}
		if resp.ResponseType == model.ResponseAckTask {
			err := c.subscriber.SetUnsubscribe(resp.TaskID)
			if err != nil {
				log.Println(err)
			}
			log.Println("Unsubscribed from task", resp.TaskID)
		}

		// serialize
		b, err := json.Marshal(&resp)
		if err != nil {
			log.Fatal(err)
		}

		// publish
		c.publisher.Send(topic+":"+string(b), 0)
	}
}

func (c *zmqClient) monitor() {

	pubMonitorAddr := "inproc://pub-monitor.rep"
	err := c.publisher.Monitor(pubMonitorAddr, zmq.EVENT_CONNECTED)
	if err != nil {
		log.Println(err)
		return
	}

	c.pubMonitor, err = zmq.NewSocket(zmq.PAIR)
	if err != nil {
		log.Println(err)
		return
	}

	err = c.pubMonitor.Connect(pubMonitorAddr)
	if err != nil {
		log.Println(err)
		return
	}

	//defer close(chMsg)
	go func() {
		for {
			eventType, eventAddr, _, err := c.pubMonitor.RecvEvent(0)
			if err != nil {
				fmt.Printf("s.RecvEvent: %s", err) // TODO send this to manager
				continue
			}
			log.Printf("Event %s %s", eventType, eventAddr)
			// send to worker
			c.pipe.RequestCh <- model.Request{Topic: model.RequestTargetAdvertisement}
		}
	}()

}

func (c *zmqClient) close() {
	log.Println("Closing ZeroMQ sockets...")

	// close subscriber
	err := c.subscriber.Close()
	if err != nil {
		log.Println(err)
	}

	// close publisher monitor
	c.pubMonitor.SetLinger(0)
	err = c.pubMonitor.Close()
	if err != nil {
		log.Println(err)
	}

	// close publisher
	err = c.publisher.Close()
	if err != nil {
		log.Println(err)
	}

}
