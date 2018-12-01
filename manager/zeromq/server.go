package zeromq

import (
	"fmt"
	"log"
	"os"
	"strings"

	"code.linksmart.eu/dt/deployment-tool/manager/model"
	zmq "github.com/pebbe/zmq4"
)

const (
	EnvDisableAuth = "DISABLE_AUTH" // disable authentication completely
)

type zmqClient struct {
	publisher  *zmq.Socket
	subscriber *zmq.Socket

	Pipe model.Pipe
}

func StartServer(pubEndpoint, subEndpoint string) (*zmqClient, error) {
	log.Printf("Using ZeroMQ v%v", strings.Replace(fmt.Sprint(zmq.Version()), " ", ".", -1))

	c := &zmqClient{
		Pipe: model.NewPipe(),
	}

	var err error
	var serverSecret string
	if evalEnv(EnvDisableAuth) {
		log.Println("WARNING: AUTHENTICATION HAS BEEN DISABLED MANUALLY.")
	} else {
		//  Start authentication engine
		zmq.AuthSetVerbose(true)
		zmq.AuthStart()

		// load keys
		serverSecret, err = loadServerKey()
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
	if !evalEnv(EnvDisableAuth) {
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
	if !evalEnv(EnvDisableAuth) {
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
			log.Printf("Error receiving event: %s", err)
		}
		// split the prefix
		parts := strings.SplitN(msg, model.TopicSeperator, 2)
		if len(parts) != 2 {
			log.Printf("Unable to parse response: %s", msg)
			continue
		}
		//log.Printf("startListener %+v", msg)
		c.Pipe.ResponseCh <- model.Message{parts[0], []byte(parts[1])}
	}
}

func (c *zmqClient) Close() error {
	log.Println("Closing ZeroMQ sockets...")

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

// evalEnv returns the boolean value of the env variable with the given key
func evalEnv(key string) bool {
	return os.Getenv(key) == "1" || os.Getenv(key) == "true" || os.Getenv(key) == "TRUE"
}
