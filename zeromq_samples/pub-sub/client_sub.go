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
	//  Socket to talk to server
	fmt.Println("Collecting updates from weather server...")
	subscriber, _ := zmq.NewSocket(zmq.SUB)
	defer subscriber.Close()
	subscriber.Connect("tcp://localhost:5556")

	filter := ""
	subscriber.SetSubscribe(filter)

	for {
		msg, _ := subscriber.Recv(0)
		fmt.Println(msg)
	}
}
