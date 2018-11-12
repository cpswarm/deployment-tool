package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"

	"code.linksmart.eu/dt/deployment-tool/model"
	zmq "github.com/pebbe/zmq4"
)

type zmqClient struct {
	subscriber *zmq.Socket
	publisher  *zmq.Socket
	pubMonitor *zmq.Socket

	pipe model.Pipe
}

func startZMQClient(subEndpoint, pubEndpoint string, pipe model.Pipe) (*zmqClient, error) {
	log.Printf("zeromq: Using ZeroMQ v%v", strings.Replace(fmt.Sprint(zmq.Version()), " ", ".", -1))
	log.Println("zeromq: Sub endpoint:", subEndpoint)
	log.Println("zeromq: Pub endpoint:", pubEndpoint)

	c := &zmqClient{
		pipe: pipe,
	}

	zmq.AuthSetVerbose(true)

	// load keys
	serverPublic, clientSecret, clientPublic, err := c.loadKeys()
	if err != nil {
		return nil, err
	}
	// socket to receive from server
	c.subscriber, err = zmq.NewSocket(zmq.SUB)
	if err != nil {
		return nil, fmt.Errorf("error creating SUB socket: %s", err)
	}
	c.subscriber.ClientAuthCurve(serverPublic, clientPublic, clientSecret)
	err = c.subscriber.Connect(subEndpoint)
	if err != nil {
		return nil, fmt.Errorf("error connecting to SUB endpoint: %s", err)
	}
	// socket to send to server
	c.publisher, err = zmq.NewSocket(zmq.PUB)
	if err != nil {
		return nil, fmt.Errorf("error creating PUB socket: %s", err)
	}
	c.publisher.ClientAuthCurve(serverPublic, clientPublic, clientSecret)
	err = c.publisher.Connect(pubEndpoint)
	if err != nil {
		return nil, fmt.Errorf("error connecting to PUB endpoint: %s", err)
	}

	c.monitor()
	go c.startListener()
	go c.startResponder()
	go c.startOperator()

	return c, nil
}

func (c *zmqClient) startListener() {
	for {
		msg, err := c.subscriber.Recv(0)
		if err != nil {
			log.Println("zeromq: Error receiving event:", err)
			continue
		}
		// split the prefix
		parts := strings.SplitN(msg, model.TopicSeperator, 2)
		if len(parts) != 2 {
			log.Println("zeromq: Error parsing event.") // TODO send to manager
			continue
		}
		// send to worker
		c.pipe.RequestCh <- model.Message{parts[0], []byte(parts[1])}
	}
}

func (c *zmqClient) startResponder() {
	for resp := range c.pipe.ResponseCh {
		_, err := c.publisher.Send(resp.Topic+":"+string(resp.Payload), 0)
		if err != nil {
			log.Println("zeromq: Error sending event:", err)
		}
	}
}

func (c *zmqClient) startOperator() {
	for op := range c.pipe.OperationCh {
		// on-demand subscription
		if op.Topic == model.OperationSubscribe {
			topic := string(op.Payload) + model.TopicSeperator
			err := c.subscriber.SetSubscribe(topic)
			if err != nil {
				log.Printf("zeromq: Error subscribing: %s", err)
			}
			log.Println("zeromq: Subscribed to", topic)
		}
		if op.Topic == model.OperationUnsubscribe {
			topic := string(op.Payload) + model.TopicSeperator
			err := c.subscriber.SetUnsubscribe(topic)
			if err != nil {
				log.Printf("zeromq: Error unsubscribing: %s", err)
			}
			log.Println("zeromq: Unsubscribed from", topic)
		}
	}
}

func (c *zmqClient) monitor() {

	pubMonitorAddr := "inproc://pub-monitor.rep"
	err := c.publisher.Monitor(pubMonitorAddr, zmq.EVENT_CONNECTED|zmq.EVENT_DISCONNECTED)
	if err != nil {
		log.Printf("zeromq: Error starting monitor: %s", err)
		return
	}

	c.pubMonitor, err = zmq.NewSocket(zmq.PAIR)
	if err != nil {
		log.Printf("zeromq: Error creating monitor socket: %s", err)
		return
	}

	err = c.pubMonitor.Connect(pubMonitorAddr)
	if err != nil {
		log.Printf("zeromq: Error connecting minitor socket: %s", err)
		return
	}

	go func() {
		for {
			eventType, eventAddr, _, err := c.pubMonitor.RecvEvent(0)
			if err != nil {
				fmt.Printf("zeromq: Error receiving monitor event: %s", err)
				continue
			}
			log.Printf("zeromq: Event %s %s", eventType, eventAddr)
			switch eventType {
			case zmq.EVENT_CONNECTED:
				// send to worker
				time.Sleep(time.Second) // solves missing pub on slow connections
				c.pipe.RequestCh <- model.Message{Topic: model.PipeConnected}
			case zmq.EVENT_DISCONNECTED:
				// send to worker
				c.pipe.RequestCh <- model.Message{Topic: model.PipeDisconnected}
			}
		}
	}()

}

func (c *zmqClient) close() {
	log.Println("zeromq: Shutting down...")

	// close subscriber
	err := c.subscriber.Close()
	if err != nil {
		log.Printf("zeromq: Error closing sub socket: %s", err)
	}

	// close publisher monitor
	c.pubMonitor.SetLinger(0)
	err = c.pubMonitor.Close()
	if err != nil {
		log.Printf("zeromq: Error closing monitor socket: %s", err)
	}

	// close publisher
	err = c.publisher.Close()
	if err != nil {
		log.Printf("zeromq: Error closing pub socket: %s", err)
	}

}

const (
	EnvPrivateKey = "PRIVATE_KEY"
	EnvPublicKey  = "PUBLIC_KEY"
	EnvManagerKey = "MANAGER_KEY"
)

func (c *zmqClient) loadKeys() (string, string, string, error) {
	if os.Getenv(EnvPrivateKey) == "" || os.Getenv(EnvPublicKey) == "" || os.Getenv(EnvManagerKey) == "" {
		log.Printf("%s=%s", EnvPrivateKey, os.Getenv(EnvPrivateKey))
		log.Printf("%s=%s", EnvPublicKey, os.Getenv(EnvPublicKey))
		log.Printf("%s=%s", EnvManagerKey, os.Getenv(EnvManagerKey))
		return "", "", "", fmt.Errorf("one or more variables are not set")
	}

	serverPublic, err := ioutil.ReadFile(os.Getenv(EnvManagerKey))
	if err != nil {
		return "", "", "", fmt.Errorf("error reading server public key: %s", err)
	}

	clientSecret, err := ioutil.ReadFile(os.Getenv(EnvPrivateKey))
	if err != nil {
		return "", "", "", fmt.Errorf("error reading client private key: %s", err)
	}

	clientPublic, err := ioutil.ReadFile(os.Getenv(EnvPublicKey))
	if err != nil {
		return "", "", "", fmt.Errorf("error reading client public key: %s", err)
	}

	return string(serverPublic), string(clientSecret), string(clientPublic), nil
}
