package main

import (
	"log"
	"os"
	"os/signal"
	"time"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("started deployment agent")
	defer log.Println("bye.")

	zmqClient, err := StartZMQClient("tcp://localhost:5556", "tcp://localhost:5557")
	if err != nil {
		log.Fatalf("Error starting ZeroMQ client: %s", err)
	}
	defer zmqClient.Close()

	taskProcessor(zmqClient)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, os.Kill)
	<-sig
}

func taskProcessor(c *ZMQClient) {

	for task := range c.TaskCh {
		log.Printf("taskProcessor: %+v", task)
		responseBatchCollector(task, time.Duration(3)*time.Second, c.ResponseCh)
	}

}
