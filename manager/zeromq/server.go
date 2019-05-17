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
	EnvDisableAuth = "DISABLE_AUTH" // disable authentication completely
	EnvPrivateKey  = "PRIVATE_KEY"
	EnvPublicKey   = "PUBLIC_KEY"

	DefaultPrivateKeyPath = "./manager.key"
	DefaultPublicKeyPath  = "./manager.pub"
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
		err = zmq.AuthStart()
		if err != nil {
			return nil, fmt.Errorf("error starting auth: %s", err)
		}

		// load key pair
		serverSecret, err = ReadKeyFile(os.Getenv(EnvPrivateKey), DefaultPrivateKeyPath)
		if err != nil {
			return nil, fmt.Errorf("error reading private key file: %s", err)
		}
		serverSecret, err = DecodeKey(serverSecret)
		if err != nil {
			return nil, fmt.Errorf("error decoding key: %s", err)
		}

		c.PublicKey, err = ReadKeyFile(os.Getenv(EnvPublicKey), DefaultPublicKeyPath)
		if err != nil {
			return nil, fmt.Errorf("error reading public key file: %s", err)
		}

	}

	// socket to publish to clients
	c.publisher, err = zmq.NewSocket(zmq.PUB)
	if err != nil {
		return nil, fmt.Errorf("error creating PUB socket: %s", err)
	}
	if !env.Eval(EnvDisableAuth) {
		err = c.publisher.ServerAuthCurve(DomainAll, serverSecret)
		if err != nil {
			return nil, fmt.Errorf("error adding server key to PUB socket: %s", err)
		}
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
		err = c.subscriber.ServerAuthCurve(DomainAll, serverSecret)
		if err != nil {
			return nil, fmt.Errorf("error adding server key to SUB socket: %s", err)
		}
	}
	err = c.subscriber.Bind(subEndpoint)
	if err != nil {
		return nil, fmt.Errorf("error connecting to SUB endpoint: %s", err)
	}

	go c.startPublisher()
	go c.startListener()
	go c.startOperator()

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

func (c *zmqClient) startOperator() {
operations:
	for op := range c.Pipe.OperationCh {
		switch op.Type {
		case model.OperationAuthAdd:
			keys, err := c.decodeKeys(op.Body)
			if err != nil {
				log.Printf("zeromq: %s", err)
				continue operations
			}
			zmq.AuthCurveAdd(DomainAll, keys...)
			log.Println("zeromq: Added client keys:", len(keys))
		case model.OperationAuthRemove:
			keys, err := c.decodeKeys(op.Body)
			if err != nil {
				log.Printf("zeromq: %s", err)
				continue operations
			}
			zmq.AuthCurveRemove(DomainAll, keys...)
			log.Println("zeromq: Removed client keys:", len(keys))
		}
	}
}

func (c *zmqClient) decodeKeys(body interface{}) ([]string, error) {
	m, ok := body.(map[string]string)
	if !ok {
		return nil, fmt.Errorf("interface conversion error: interface {} is not map[string]string")
	}
	var keys []string
	for k, v := range m {
		decoded, err := DecodeKey(v)
		if err != nil {
			return nil, fmt.Errorf("unable to decode key (%s) from client %s: %s", v, k, err)
		}
		keys = append(keys, decoded)
	}
	return keys, nil
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
