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
	reqTopic = "REQ-"
	// <arch>.<os>.<distro>.<os_version>.<hw>.<hw_version>
	ackTopic = "ACK-"
	resTopic = "RES-"
)

type ZMQClient struct {
	publisher  *zmq.Socket
	subscriber *zmq.Socket

	TaskCh     chan model.Task
	ResponseCh chan model.BatchResponse
}

func StartZMQClient(pubEndpoint, subEndpoint string) (*ZMQClient, error) {
	log.Printf("Using ZeroMQ v%v", strings.Replace(fmt.Sprint(zmq.Version()), " ", ".", -1))

	c := &ZMQClient{
		TaskCh:     make(chan model.Task),
		ResponseCh: make(chan model.BatchResponse),
	}

	var err error
	// socket to publish to clients
	c.publisher, err = zmq.NewSocket(zmq.PUB)
	if err != nil {
		return nil, fmt.Errorf("error creating PUB socket: %s", err)
	}
	err = c.publisher.Bind(pubEndpoint)
	if err != nil {
		return nil, fmt.Errorf("error binding to PUB endpoint: %s", err)
	}

	// socket to receive from clients
	c.subscriber, err = zmq.NewSocket(zmq.SUB)
	if err != nil {
		return nil, fmt.Errorf("error creating SUB socket: %s", err)
	}
	err = c.subscriber.Bind(subEndpoint)
	if err != nil {
		return nil, fmt.Errorf("error connecting to SUB endpoint: %s", err)
	}

	go c.startTaskPublisher()
	go c.startListener()

	return c, nil
}

func (c *ZMQClient) startTaskPublisher() {
	// sender
	for task := range c.TaskCh {
		//log.Printf("startTaskPublisher %+v", task)
		b, err := json.Marshal(&task)
		if err != nil {
			log.Fatal(err)
		}
		c.publisher.Send(reqTopic+string(b), 0)
	}
}

func (c *ZMQClient) startListener() {
	// listener
	c.subscriber.SetSubscribe(ackTopic)
	c.subscriber.SetSubscribe(resTopic)

	for {
		msg, err := c.subscriber.Recv(0)
		if err != nil {
			log.Fatal(err)
		}

		// drop the filter
		msg = strings.TrimPrefix(msg, ackTopic)
		msg = strings.TrimPrefix(msg, resTopic)
		// de-serialize
		var response model.BatchResponse
		err = json.Unmarshal([]byte(msg), &response)
		if err != nil {
			log.Fatal(err)
		}
		//log.Printf("startListener %+v", msg)
		c.ResponseCh <- response
	}
}

func (c *ZMQClient) Close() error {
	log.Println("Closing ZeroMQ sockets...")

	err := c.subscriber.Close()
	if err != nil {
		return err
	}

	err = c.publisher.Close()
	if err != nil {
		return err
	}

	return nil
}
