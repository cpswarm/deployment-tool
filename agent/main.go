package main

import (
	"log"
	"os"
	"os/signal"
	"time"
)

func main() {
	log.Println("started deployment agent")
	defer log.Println("bye.")

	zmqClient, err := StartZMQClient("tcp://localhost:5556", "tcp://localhost:5557")
	if err != nil {
		log.Fatalf("Error starting ZeroMQ client: %s", err)
	}
	defer zmqClient.Close()

	requestProcessor(zmqClient)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, os.Kill)
	<-sig
}

func requestProcessor(c *ZMQClient) {

	for req := range c.RequestCh {
		log.Printf("requestProcessor: %+v", req)
		responseBatchCollector(req.Commands, time.Duration(3)*time.Second, c.ResponseCh)
	}

}
