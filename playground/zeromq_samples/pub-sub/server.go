//
//  Weather update server.
//  Binds PUB socket to tcp://*:5556
//  Publishes random weather updates
//

package main

import (
	"fmt"
	"os"
	"os/signal"
	"time"

	zmq "github.com/pebbe/zmq4"
)

func main() {
	// socket to publish to clients
	publisher, _ := zmq.NewSocket(zmq.PUB)
	defer publisher.Close()
	publisher.Bind("tcp://*:5556")
	// socket to receive from clients
	subscriber, _ := zmq.NewSocket(zmq.SUB)
	defer subscriber.Close()
	subscriber.Bind("tcp://*:5557")

	go func() {
		filter := "ACK"
		subscriber.SetSubscribe(filter)

		for {
			msg, _ := subscriber.Recv(0)
			fmt.Println(msg)
		}
	}()

	go func() {
		for {
			publisher.Send(time.Now().String(), 0)
			time.Sleep(1 * time.Second)
		}
	}()

	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, os.Kill)
	<-c
	fmt.Println("bye")
}
