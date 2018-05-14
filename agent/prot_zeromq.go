package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"code.linksmart.eu/dt/deployment-tool/model"
	zmq "github.com/pebbe/zmq4"
)

const reqPrefix = "REQ -"

// <arch>.<os>.<distro>.<os_version>.<hw>.<hw_version>
const ackPrefix = "ACK -"
const resPrefix = "RES -"

type ZMQClient struct {
	subscriber *zmq.Socket
	publisher  *zmq.Socket

	RequestCh  chan model.Task
	ResponseCh chan BatchResponse
}

func StartZMQClient(subEndpoint, pubEndpoint string) (*ZMQClient, error) {
	c := &ZMQClient{
		RequestCh:  make(chan model.Task),
		ResponseCh: make(chan BatchResponse),
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

	filter := ""
	go c.startListener(filter)
	go c.startResponder()

	return c, nil
}

func (c *ZMQClient) startListener(filter string) {

	c.subscriber.SetSubscribe(filter)
	for {
		msg, err := c.subscriber.Recv(0)
		if err != nil {
			log.Fatalln(err)
		}
		task, err := c.requestHandler(msg)
		c.RequestCh <- task
		c.publisher.Send("ACK", 0)
	}
}

func (c *ZMQClient) startResponder() {
	for resp := range c.ResponseCh {
		log.Printf("Batch: %+v", resp)
		b, err := json.Marshal(resp)
		if err != nil {
			log.Fatal(err)
		}
		_, err = c.publisher.Send("RES -- "+string(b), 0)
		if err != nil {
			log.Fatal(err)
		}
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

func (c *ZMQClient) requestHandler(msg string) (model.Task, error) {
	fmt.Println("requestHandler: ", msg)
	// drop the filter
	msg = strings.TrimPrefix(msg, reqPrefix)
	// deserialize
	var task model.Task
	err := json.Unmarshal([]byte(msg), &task)
	if err != nil {
		log.Fatal(err)
	}
	c.RequestCh <- task
	// send acknowledgement msg
	c.publisher.Send(ackPrefix+msg, 0)
}

func (c *ZMQClient) responseHandler(resp *BatchResponse) {
	log.Printf("responseHandler: %+v", resp)

	// serialize
	b, err := json.Marshal(resp)
	if err != nil {
		log.Fatal(err)
	}

	c.publisher.Send(resPrefix+string(b), 0)
}
