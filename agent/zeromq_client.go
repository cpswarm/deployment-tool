package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"code.linksmart.eu/dt/deployment-tool/manager/env"
	"code.linksmart.eu/dt/deployment-tool/manager/model"
	"code.linksmart.eu/dt/deployment-tool/manager/zeromq"
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

func startZMQClient(conf *zeromqServer, clientPublic string, pipe model.Pipe) (*zmqClient, error) {
	log.Printf("zeromq: Using ZeroMQ v%v", strings.Replace(fmt.Sprint(zmq.Version()), " ", ".", -1))
	subEndpoint := fmt.Sprintf("%s:%d", conf.host, conf.SubPort)
	pubEndpoint := fmt.Sprintf("%s:%d", conf.host, conf.PubPort)
	log.Println("zeromq: Sub endpoint:", subEndpoint)
	log.Println("zeromq: Pub endpoint:", pubEndpoint)
	log.Println("zeromq: Server public key:", conf.PublicKey)
	log.Println("zeromq: Client public key:", clientPublic)

	c := &zmqClient{
		pipe: pipe,
	}

	var err error

	// load keys
	var clientSecret, serverPublic string
	if env.Eval(EnvDisableAuth) {
		log.Println("WARNING: AUTHENTICATION HAS BEEN DISABLED MANUALLY.")
	} else {
		zmq.AuthSetVerbose(true)
		clientSecret, err = zeromq.ReadKeyFile(os.Getenv(EnvPrivateKey), DefaultPrivateKeyPath)
		if err != nil {
			return nil, err
		}
		// decode
		clientSecret, err = zeromq.DecodeKey(clientSecret)
		if err != nil {
			return nil, err
		}
		clientPublic, err = zeromq.DecodeKey(clientPublic)
		if err != nil {
			return nil, err
		}
		serverPublic, err = zeromq.DecodeKey(conf.PublicKey)
		if err != nil {
			return nil, err
		}
	}
	// socket to receive from server
	c.subscriber, err = zmq.NewSocket(zmq.SUB)
	if err != nil {
		return nil, fmt.Errorf("error creating SUB socket: %s", err)
	}
	if !env.Eval(EnvDisableAuth) {
		c.subscriber.ClientAuthCurve(serverPublic, clientPublic, clientSecret)
	}
	c.subscriber.SetReconnectIvlMax(MaxReconnectInterval)
	err = c.subscriber.Connect(pubEndpoint)
	if err != nil {
		return nil, fmt.Errorf("error connecting to SUB endpoint: %s", err)
	}
	// socket to send to server
	c.publisher, err = zmq.NewSocket(zmq.PUB)
	if err != nil {
		return nil, fmt.Errorf("error creating PUB socket: %s", err)
	}
	if !env.Eval(EnvDisableAuth) {
		c.publisher.ClientAuthCurve(serverPublic, clientPublic, clientSecret)
	}
	c.publisher.SetReconnectIvlMax(MaxReconnectInterval)
	err = c.publisher.Connect(subEndpoint)
	if err != nil {
		return nil, fmt.Errorf("error connecting to PUB endpoint: %s", err)
	}

	startMonitor, err := c.setupMonitor()
	if err != nil {
		return nil, fmt.Errorf("error starting monitor: %s", err)
	}
	go startMonitor()

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
		if env.Debug {
			log.Printf("zeromq: Received %d bytes", len([]byte(msg)))
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
		length, err := c.publisher.Send(resp.Topic+":"+string(resp.Payload), 0)
		if err != nil {
			log.Println("zeromq: Error sending event:", err)
		}
		if env.Debug {
			log.Printf("zeromq: Sent %d bytes", length)
		}
	}
}

func (c *zmqClient) startOperator() {
	for op := range c.pipe.OperationCh {
		switch op.Type {
		case model.OperationSubscribe:
			topic := op.Body.(string) + model.TopicSeperator
			err := c.subscriber.SetSubscribe(topic)
			if err != nil {
				log.Printf("zeromq: Error subscribing: %s", err)
			}
			log.Println("zeromq: Subscribed to", topic)
		case model.OperationUnsubscribe:
			topic := op.Body.(string) + model.TopicSeperator
			err := c.subscriber.SetUnsubscribe(topic)
			if err != nil {
				log.Printf("zeromq: Error unsubscribing: %s", err)
			}
			log.Println("zeromq: Unsubscribed from", topic)
		}
	}
}

func (c *zmqClient) setupMonitor() (func(), error) {

	pubMonitorAddr := "inproc://pub-monitor.rep"
	err := c.publisher.Monitor(pubMonitorAddr, zmq.EVENT_CONNECTED|zmq.EVENT_DISCONNECTED|zmq.EVENT_ALL)
	if err != nil {
		return nil, fmt.Errorf("error registering monitor: %s", err)
	}

	c.pubMonitor, err = zmq.NewSocket(zmq.PAIR)
	if err != nil {
		return nil, fmt.Errorf("error creating monitor socket: %s", err)
	}

	err = c.pubMonitor.Connect(pubMonitorAddr)
	if err != nil {
		return nil, fmt.Errorf("error connecting minitor socket: %s", err)
	}

	return func() {
		for {
			eventType, eventAddr, eventValue, err := c.pubMonitor.RecvEvent(0)
			if err != nil {
				fmt.Printf("zeromq: Error receiving monitor event: %s", err)
				continue
			}
			if env.Debug || eventType == zmq.EVENT_CONNECTED || eventType == zmq.EVENT_DISCONNECTED {
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
				c.pipe.RequestCh <- model.Message{Topic: model.PipeConnected}
			case zmq.EVENT_DISCONNECTED:
				// send to worker
				c.pipe.RequestCh <- model.Message{Topic: model.PipeDisconnected}
			}
		}
	}, nil
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

func writeNewKeys() error {
	privateKeyPath := os.Getenv(EnvPrivateKey)
	publicKeyPath := os.Getenv(EnvPublicKey)

	if privateKeyPath == "" {
		privateKeyPath = DefaultPrivateKeyPath
		log.Printf("zeromq: %s not set. Using default path: %s", EnvPrivateKey, DefaultPrivateKeyPath)
	}
	if publicKeyPath == "" {
		publicKeyPath = DefaultPublicKeyPath
		log.Printf("zeromq: %s not set. Using default path: %s", EnvPublicKey, DefaultPublicKeyPath)
	}

	err := zeromq.NewCurveKeypair(privateKeyPath, publicKeyPath)
	if err != nil {
		if os.IsExist(err) {
			log.Printf("zeromq: Key file already exists.")
			return nil
		}
		return fmt.Errorf("error creating key pair: %s", err)
	}

	log.Printf("zeromq: Created new key pair.")
	return nil
}
