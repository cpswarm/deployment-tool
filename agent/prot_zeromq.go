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

type zmqClient struct {
	subscriber *zmq.Socket
	publisher  *zmq.Socket

	pipe model.Pipe
}

func startZMQClient(subEndpoint, pubEndpoint string) (*zmqClient, error) {
	log.Printf("Using ZeroMQ v%v", strings.Replace(fmt.Sprint(zmq.Version()), " ", ".", -1))
	log.Println("Sub endpoint:", subEndpoint)
	log.Println("Pub endpoint:", pubEndpoint)

	c := &zmqClient{
		pipe: model.NewPipe(),
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

	// TODO subscribe to the next update, to avoid getting resent tasks
	topic := reqTopic
	go c.startListener(topic)
	go c.startResponder()

	return c, nil
}

func (c *zmqClient) startListener(topic string) {

	c.subscriber.SetSubscribe(topic)
	for {
		msg, err := c.subscriber.Recv(0)
		if err != nil {
			log.Println("ERROR:", err)
			continue
		}
		// drop the filter
		msg = strings.TrimPrefix(msg, reqTopic)
		// deserialize
		var task model.Task
		err = json.Unmarshal([]byte(msg), &task)
		if err != nil {
			log.Fatal(err)
		}
		// send to worker
		c.pipe.TaskCh <- task
	}
}

func (c *zmqClient) startResponder() {
	for resp := range c.pipe.ResponseCh {
		// set publishing topic
		topic := resTopic
		if resp.ResponseType == model.ResponseACK {
			topic = ackTopic
		}
		// serialize
		b, err := json.Marshal(resp)
		if err != nil {
			log.Fatal(err)
		}
		// publish
		c.publisher.Send(topic+string(b), 0)
	}
}

func (c *zmqClient) close() error {
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
