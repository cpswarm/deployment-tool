package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"

	"code.linksmart.eu/dt/deployment-tool/manager/model"
	zmq "github.com/pebbe/zmq4"
)

const (
	MaxReconnectInterval = 30 * time.Second
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

	var err error

	// load keys
	var serverPublic, clientSecret, clientPublic string
	if evalEnv(EnvDisableAuth) {
		log.Println("WARNING: AUTHENTICATION HAS BEEN DISABLED MANUALLY.")
	} else {
		zmq.AuthSetVerbose(true)
		serverPublic, clientSecret, clientPublic, err = c.loadKeys()
		if err != nil {
			return nil, err
		}
	}
	// socket to receive from server
	c.subscriber, err = zmq.NewSocket(zmq.SUB)
	if err != nil {
		return nil, fmt.Errorf("error creating SUB socket: %s", err)
	}
	if !evalEnv(EnvDisableAuth) {
		c.subscriber.ClientAuthCurve(serverPublic, clientPublic, clientSecret)
	}
	c.subscriber.SetReconnectIvlMax(MaxReconnectInterval)
	err = c.subscriber.Connect(subEndpoint)
	if err != nil {
		return nil, fmt.Errorf("error connecting to SUB endpoint: %s", err)
	}
	// socket to send to server
	c.publisher, err = zmq.NewSocket(zmq.PUB)
	if err != nil {
		return nil, fmt.Errorf("error creating PUB socket: %s", err)
	}
	if !evalEnv(EnvDisableAuth) {
		c.publisher.ClientAuthCurve(serverPublic, clientPublic, clientSecret)
	}
	c.publisher.SetReconnectIvlMax(MaxReconnectInterval)
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
	err := c.publisher.Monitor(pubMonitorAddr, zmq.EVENT_CONNECTED|zmq.EVENT_DISCONNECTED|zmq.EVENT_ALL)
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
			eventType, eventAddr, eventValue, err := c.pubMonitor.RecvEvent(0)
			if err != nil {
				fmt.Printf("zeromq: Error receiving monitor event: %s", err)
				continue
			}
			if evalEnv(EnvDebug) || eventType == zmq.EVENT_CONNECTED || eventType == zmq.EVENT_DISCONNECTED {
				log.Printf("zeromq: Event %s %s %d", eventType, eventAddr, eventValue)
			}
			go func() {
				if eventValue == 400 {
					time.Sleep(1 * time.Second)
					log.Println("zeromq: Error 400 from server. Client not authenticated?")
					log.Println("Will exit in 30s...")
					time.Sleep(MaxReconnectInterval)
					os.Exit(1)
				}
			}()
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

func (c *zmqClient) loadKeys() (string, string, string, error) {
	privateKeyPath := os.Getenv(EnvPrivateKey)
	publicKeyPath := os.Getenv(EnvPublicKey)
	managerKeyPath := os.Getenv(EnvManagerPublicKey)
	managerKeyStr := os.Getenv(EnvManagerPublicKeyStr)

	if privateKeyPath == "" {
		privateKeyPath = DefaultPrivateKeyPath
		log.Printf("zeromq: %s not set. Using default path: %s", EnvPrivateKey, DefaultPrivateKeyPath)
	}
	if publicKeyPath == "" {
		publicKeyPath = DefaultPublicKeyPath
		log.Printf("zeromq: %s not set. Using default path: %s", EnvPublicKey, DefaultPublicKeyPath)
	}
	if managerKeyPath == "" {
		managerKeyPath = DefaultManagerKeyPath
		log.Printf("zeromq: %s not set. Using default path: %s", EnvManagerPublicKey, DefaultManagerKeyPath)
	}

	clientSecret, err := ioutil.ReadFile(privateKeyPath)
	if err != nil {
		return "", "", "", fmt.Errorf("error reading client private key: %s", err)
	}
	clientSecret, err = base64.StdEncoding.DecodeString(string(clientSecret))
	if err != nil {
		return "", "", "", fmt.Errorf("error decoding client private key: %s", err)
	}

	clientPublic, err := ioutil.ReadFile(publicKeyPath)
	if err != nil {
		return "", "", "", fmt.Errorf("error reading client public key: %s", err)
	}
	log.Println("zeromq: Public key ->", string(bytes.TrimSpace(clientPublic)))
	clientPublic, err = base64.StdEncoding.DecodeString(string(clientPublic))
	if err != nil {
		return "", "", "", fmt.Errorf("error decoding client public key: %s", err)
	}

	var serverPublic []byte
	if managerKeyStr == "" {
		serverPublic, err = ioutil.ReadFile(managerKeyPath)
		if err != nil {
			return "", "", "", fmt.Errorf("error reading server public key: %s", err)
		}
	} else {
		// take key directly from variable
		log.Printf("zeromq: Read manager public key from env variable.")
		serverPublic = []byte(managerKeyStr)
	}
	log.Printf("zeromq: manager public key: %s", string(serverPublic))
	serverPublic, err = base64.StdEncoding.DecodeString(string(serverPublic))
	if err != nil {
		return "", "", "", fmt.Errorf("error decoding manager public key: %s", err)
	}

	return string(bytes.TrimSpace(serverPublic)), string(bytes.TrimSpace(clientSecret)), string(bytes.TrimSpace(clientPublic)), nil
}
