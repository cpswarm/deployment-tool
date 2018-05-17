package main

import (
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"time"

	"code.linksmart.eu/dt/deployment-tool/model"
	zmq "github.com/pebbe/zmq4"
	uuid "github.com/satori/go.uuid"
)

const (
	reqTopic = "REQ-"
	// <arch>.<os>.<distro>.<os_version>.<hw>.<hw_version>
	ackTopic = "ACK-"
	resTopic = "RES-"
)

func main() {
	log.Println("started deployment manager")
	defer log.Println("bye.")

	// socket to publish to clients
	publisher, _ := zmq.NewSocket(zmq.PUB)
	defer publisher.Close()
	publisher.Bind("tcp://*:5556")
	// socket to receive from clients
	subscriber, _ := zmq.NewSocket(zmq.SUB)
	defer subscriber.Close()
	subscriber.Bind("tcp://*:5557")

	// listener
	go func() {
		subscriber.SetSubscribe(ackTopic)
		subscriber.SetSubscribe(resTopic)

		for {
			msg, _ := subscriber.Recv(0)
			log.Println(msg)
		}
	}()

	// sender
	go func() {
		for i := 0; i < 3; i++ {
			task := model.Task{
				Commands: []string{"pwdd", "pwd", "pwdd"},
				Time:     time.Now().Unix(),
				ID:       uuid.NewV4().String(),
			}
			log.Printf("Sending task: %+v", task)
			b, err := json.Marshal(&task)
			if err != nil {
				log.Fatal(err)
			}
			publisher.Send(reqTopic+string(b), 0)
			time.Sleep(3 * time.Second)
		}
		os.Exit(0)
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, os.Kill)
	<-sig
}
