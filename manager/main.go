package main

import (
	"log"
	"os"
	"os/signal"

	"code.linksmart.eu/dt/deployment-tool/model"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("started deployment manager")
	defer log.Println("bye.")

	zmqClient, err := startZMQClient("tcp://*:5556", "tcp://*:5557")
	if err != nil {
		log.Fatalf("Error starting ZeroMQ client: %s", err)
	}
	defer zmqClient.close()

	m, err := newManager(zmqClient.pipe)
	if err != nil {
		log.Fatal(err)
	}
	go m.processResponses()

	// add dummy targets
	m.targets["target1"] = &model.Target{}

	go startRESTAPI(":8080", m)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, os.Kill)
	<-sig
}
