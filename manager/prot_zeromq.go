package main

import (
	"fmt"
	"log"
	"strings"

	"code.linksmart.eu/dt/deployment-tool/model"
	zmq "github.com/pebbe/zmq4"
)

type zmqClient struct {
	publisher  *zmq.Socket
	subscriber *zmq.Socket

	pipe model.Pipe
}

func startZMQClient(pubEndpoint, subEndpoint string) (*zmqClient, error) {
	log.Printf("Using ZeroMQ v%v", strings.Replace(fmt.Sprint(zmq.Version()), " ", ".", -1))

	c := &zmqClient{
		pipe: model.NewPipe(),
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

	go c.startPublisher()
	go c.startListener()

	err = c.subscriber.SetSubscribe("")
	if err != nil {
		return nil, fmt.Errorf("error subscribing: %s", err)
	}

	return c, nil
}

func (c *zmqClient) startPublisher() {
	for request := range c.pipe.RequestCh {
		_, err := c.publisher.Send(request.Topic+":"+string(request.Payload), 0)
		if err != nil {
			log.Printf("error publishing: %s", err)
		}
	}
}

func (c *zmqClient) startListener() {
	for {
		msg, err := c.subscriber.Recv(0)
		if err != nil {
			log.Fatal(err)
		}
		// split the prefix
		parts := strings.SplitN(msg, ":", 2)
		if len(parts) != 2 {
			log.Printf("Unable to parse response: %s", msg)
			continue
		}
		//log.Printf("startListener %+v", msg)
		c.pipe.ResponseCh <- model.Message{parts[0], []byte(parts[1])}
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
