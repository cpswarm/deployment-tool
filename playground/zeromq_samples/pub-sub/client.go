//
//  Weather update client.
//  Connects SUB socket to tcp://localhost:5556
//  Collects weather updates and finds avg temp in zipcode
//

package main

import (
	"fmt"

	zmq "github.com/pebbe/zmq4"
)

func main() {
	// socket to receive from server
	subscriber, _ := zmq.NewSocket(zmq.SUB)
	defer subscriber.Close()
	subscriber.Connect("tcp://localhost:5556")
	// socket to send to server
	publisher, _ := zmq.NewSocket(zmq.PUB)
	defer publisher.Close()
	publisher.Connect("tcp://localhost:5557")

	filter := ""
	subscriber.SetSubscribe(filter)

	for {
		msg, _ := subscriber.Recv(0)
		fmt.Println(msg)
		publisher.Send("ACK -- "+msg, 0)
	}
}
