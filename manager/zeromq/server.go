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
	EnvPrivateKey = "PRIVATE_KEY"
	EnvPublicKey  = "PUBLIC_KEY"

	DefaultPrivateKeyPath = "./manager.key"
	DefaultPublicKeyPath  = "./manager.pub"
)

type zmqClient struct {
	publisher  *zmq.Socket
	subscriber *zmq.Socket
	conf       model.ZeromqServerInfo

	Pipe model.Pipe
}

func SetupServer(pubPort, subPort string, keys map[string]string) (*zmqClient, error) {
	log.Printf("zeromq: Using v%v", strings.Replace(fmt.Sprint(zmq.Version()), " ", ".", -1))

	c := &zmqClient{
		conf: model.ZeromqServerInfo{
			PubPort: pubPort,
			SubPort: subPort,
		},
		Pipe: model.NewPipe(),
	}

	pubEndpoint, subEndpoint := "tcp://*:"+pubPort, "tcp://*:"+subPort
	log.Println("zeromq: Pub endpoint:", pubEndpoint)
	log.Println("zeromq: Sub endpoint:", subEndpoint)

	var err error
	var serverSecret string

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

	c.conf.PublicKey, err = ReadKeyFile(os.Getenv(EnvPublicKey), DefaultPublicKeyPath)
	if err != nil {
		return nil, fmt.Errorf("error reading public key file: %s", err)
	}

	// add client keys
	err = c.addKeys(keys)
	if err != nil {
		return nil, fmt.Errorf("error decoding key: %s", err)
	}
	log.Println("zeromq: Added client keys:", len(keys))

	// socket to publish to clients
	c.publisher, err = zmq.NewSocket(zmq.PUB)
	if err != nil {
		return nil, fmt.Errorf("error creating PUB socket: %s", err)
	}

	err = c.publisher.ServerAuthCurve(DomainAll, serverSecret)
	if err != nil {
		return nil, fmt.Errorf("error adding server key to PUB socket: %s", err)
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

	err = c.subscriber.ServerAuthCurve(DomainAll, serverSecret)
	if err != nil {
		return nil, fmt.Errorf("error adding server key to SUB socket: %s", err)
	}

	err = c.subscriber.Bind(subEndpoint)
	if err != nil {
		return nil, fmt.Errorf("error connecting to SUB endpoint: %s", err)
	}

	return c, nil
}

func (c *zmqClient) Conf() model.ZeromqServerInfo {
	return c.conf
}

func (c *zmqClient) Start() error {
	err := c.subscriber.SetSubscribe("")
	if err != nil {
		return fmt.Errorf("error subscribing: %s", err)
	}

	go c.startPublisher()
	go c.startListener()
	go c.startOperator()

	log.Println("zeromq: Started server.")
	return nil
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
			m, ok := op.Body.(map[string]string)
			if !ok {
				log.Printf("zeromq: Error converting body for OperationAuthAdd: interface{} is not map[string]string")
				continue operations
			}
			err := c.addKeys(m)
			if err != nil {
				log.Printf("zeromq: %s", err)
				continue operations
			}
			log.Println("zeromq: Added client key.")
		case model.OperationAuthRemove:
			m, ok := op.Body.(map[string]string)
			if !ok {
				log.Printf("zeromq: Error converting body for OperationAuthRemove: interface{} is not map[string]string")
				continue operations
			}
			err := c.removeKeys(m)
			if err != nil {
				log.Printf("zeromq: %s", err)
				continue operations
			}
			log.Println("zeromq: Removed client keys")
		}
	}
}

func (c *zmqClient) decodeKeys(m map[string]string) ([]string, error) {
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

func (c *zmqClient) addKeys(m map[string]string) error {
	keys, err := c.decodeKeys(m)
	if err != nil {
		return fmt.Errorf("error decoding keys: %s", err)
	}
	zmq.AuthCurveAdd(DomainAll, keys...)
	return nil
}

func (c *zmqClient) removeKeys(m map[string]string) error {
	keys, err := c.decodeKeys(m)
	if err != nil {
		return fmt.Errorf("error decoding keys: %s", err)
	}
	zmq.AuthCurveRemove(DomainAll, keys...)
	return nil
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
