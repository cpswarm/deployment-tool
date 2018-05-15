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
	subscriber *zmq.Socket
	publisher  *zmq.Socket

	RequestCh  chan model.Task
	ResponseCh chan model.BatchResponse
}

func StartZMQClient(subEndpoint, pubEndpoint string) (*ZMQClient, error) {
	log.Printf("Using ZeroMQ v%v", strings.Replace(fmt.Sprint(zmq.Version()), " ", ".", -1))

	c := &ZMQClient{
		RequestCh:  make(chan model.Task),
		ResponseCh: make(chan model.BatchResponse),
	}

	var err error
	// socket to receive from server
	c.subscriber, err = zmq.NewSocket(zmq.SUB)
	if err != nil {
		return nil, err
	}
	err = c.subscriber.Connect(subEndpoint)
	if err != nil {
		return nil, err
	}
	// socket to send to server
	c.publisher, err = zmq.NewSocket(zmq.PUB)
	if err != nil {
		return nil, err
	}
	err = c.publisher.Connect(pubEndpoint)
	if err != nil {
		return nil, err
	}

	topic := ""
	go c.startListener(topic)
	go c.startResponder()

	return c, nil
}

func (c *ZMQClient) startListener(topic string) {

	c.subscriber.SetSubscribe(topic)
	for {
		msg, err := c.subscriber.Recv(0)
		if err != nil {
			log.Fatalln(err)
		}

		// de-serialize
		task := c.requestDeserializer(msg)
		// response acknowledgement
		c.ResponseCh <- model.BatchResponse{ResponseType: model.ResponseTypeACK, TaskID: task.ID}
		// send to worker
		c.RequestCh <- task
	}
}

func (c *ZMQClient) startResponder() {
	for resp := range c.ResponseCh {
		// serialize
		msg := c.responseSerializer(&resp)

		// set publishing topic
		topic := resTopic
		if resp.ResponseType == model.ResponseTypeACK {
			topic = ackTopic
		}

		// publish
		c.publisher.Send(topic+msg, 0)
	}
}

func (c *ZMQClient) requestDeserializer(msg string) model.Task {
	fmt.Println("requestDeserializer: ", msg)
	// drop the filter
	msg = strings.TrimPrefix(msg, reqTopic)
	// deserialize
	var task model.Task
	err := json.Unmarshal([]byte(msg), &task)
	if err != nil {
		log.Fatal(err)
	}
	return task
}

func (c *ZMQClient) responseSerializer(resp *model.BatchResponse) string {
	log.Printf("responseSerializer: %+v", resp)
	// serialize
	b, err := json.Marshal(resp)
	if err != nil {
		log.Fatal(err)
	}
	return string(b)
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
