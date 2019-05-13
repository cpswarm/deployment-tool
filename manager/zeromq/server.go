package zeromq

import (
	"fmt"
	"log"
	"os"
	"strings"

	"code.linksmart.eu/dt/deployment-tool/manager/env"
	"code.linksmart.eu/dt/deployment-tool/manager/model"
	zmq "github.com/pebbe/zmq4"
)

const (
	EnvDebug       = "DEBUG"        // print debug messages
	EnvDisableAuth = "DISABLE_AUTH" // disable authentication completely
)

type zmqClient struct {
	publisher  *zmq.Socket
	subscriber *zmq.Socket

	Pipe      model.Pipe
	PublicKey string
}

func StartServer(pubEndpoint, subEndpoint string) (*zmqClient, error) {
	log.Printf("zeromq: Using v%v", strings.Replace(fmt.Sprint(zmq.Version()), " ", ".", -1))

	c := &zmqClient{
		Pipe: model.NewPipe(),
	}

	var err error
	var serverSecret string
	if env.Eval(EnvDisableAuth) {
		log.Println("zeromq: WARNING: AUTHENTICATION HAS BEEN DISABLED MANUALLY.")
	} else {
		//  Start authentication engine
		zmq.AuthSetVerbose(true)
		zmq.AuthStart()

		// load key pair
		serverSecret, err = ReadKeyFile(os.Getenv(EnvPrivateKey), DefaultPrivateKeyPath)
		if err != nil {
			return nil, err
		}
		serverSecret, err = DecodeKey(serverSecret)
		if err != nil {
			return nil, err
		}

		c.PublicKey, err = ReadKeyFile(os.Getenv(EnvPublicKey), DefaultPublicKeyPath)
		if err != nil {
			return nil, err
		}

		err = loadClientKeys()
		if err != nil {
			return nil, err
		}
	}

	// socket to publish to clients
	c.publisher, err = zmq.NewSocket(zmq.PUB)
	if err != nil {
		return nil, fmt.Errorf("error creating PUB socket: %s", err)
	}
	if !env.Eval(EnvDisableAuth) {
		c.publisher.ServerAuthCurve(DomainAll, serverSecret)
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
	if !env.Eval(EnvDisableAuth) {
		c.subscriber.ServerAuthCurve(DomainAll, serverSecret)
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
	for request := range c.Pipe.RequestCh {
		length, err := c.publisher.Send(request.Topic+":"+string(request.Payload), 0)
		if err != nil {
			log.Printf("zeromq: Error publishing: %s", err)
		}
		if env.Debug {
			log.Printf("zeromq: Sent %d bytes", length)
		}
	}
}

func (c *zmqClient) startListener() {
	for {
		msg, err := c.subscriber.Recv(0)
		if err != nil {
			log.Printf("zeromq: Error receiving event: %s", err)
		}
		if env.Debug {
			log.Printf("zeromq: Received %d bytes", len([]byte(msg)))
		}
		// split the prefix
		parts := strings.SplitN(msg, model.TopicSeperator, 2)
		if len(parts) != 2 {
			log.Printf("zeromq: Unable to parse response: %s", msg)
			continue
		}
		c.Pipe.ResponseCh <- model.Message{parts[0], []byte(parts[1])}
	}
}

func (c *zmqClient) Close() error {
	log.Println("zeromq: Closing sockets...")

	err := c.subscriber.Close()
	if err != nil {
		return err
	}

	err = c.publisher.Close()
	if err != nil {
		return err
	}

	zmq.AuthStop()

	return nil
}
