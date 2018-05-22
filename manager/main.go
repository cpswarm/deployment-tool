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

	zmqClient, err := StartZMQClient("tcp://*:5556", "tcp://*:5557")
	if err != nil {
		log.Fatalf("Error starting ZeroMQ client: %s", err)
	}
	defer zmqClient.Close()

	m, err := NewManager()
	if err != nil {
		log.Fatal(err)
	}

	// add dummy targets
	m.targets["t1"] = &model.Target{}
	//m.targets["t2"] = &model.Target{}

	go m.processResponses(zmqClient.ResponseCh)
	go m.sendTasks(zmqClient.TaskCh)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, os.Kill)
	<-sig
}
