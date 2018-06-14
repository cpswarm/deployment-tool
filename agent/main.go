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

	a := startAgent()
	defer a.close()


	zmqClient, err := startZMQClient(managerEndpoint+":5556", managerEndpoint+":5557", a.Target.ID, a.pipe)
	if err != nil {
		log.Fatalf("Error starting ZeroMQ client: %s", err)
	}
	defer zmqClient.close()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, os.Kill)
	<-sig
}
