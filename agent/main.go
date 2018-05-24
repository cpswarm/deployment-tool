package main

import (
	"log"
	"os"
	"os/signal"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("started deployment agent")
	defer log.Println("bye.")

	managerEndpoint := os.Getenv("MANAGER")

	if managerEndpoint == "" {
		managerEndpoint = "tcp://localhost"
	}

	zmqClient, err := startZMQClient(managerEndpoint+":5556", managerEndpoint+":5557")
	if err != nil {
		log.Fatalf("Error starting ZeroMQ client: %s", err)
	}
	defer zmqClient.close()

	a := newAgent(zmqClient.pipe)
	defer a.close()
	go a.startTaskProcessor()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, os.Kill)
	<-sig
}
