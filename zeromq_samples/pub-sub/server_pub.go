//
//  Weather update server.
//  Binds PUB socket to tcp://*:5556
//  Publishes random weather updates
//

package main

import (
	"bufio"
	"fmt"
	"os"

	zmq "github.com/pebbe/zmq4"
)

func main() {

	//  Prepare our publisher
	publisher, _ := zmq.NewSocket(zmq.PUB)
	defer publisher.Close()
	publisher.Bind("tcp://*:5556")
	//publisher.Bind("ipc://weather.ipc")

	for {
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("Enter text: ")
		text, _ := reader.ReadString('\n')

		publisher.Send(text, 0)
	}
}
