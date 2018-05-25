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
	//m.targets["target1"] = &model.Target{}
	m.targets["iot-raspizero-1"] = &model.Target{
		ID:   "iot-raspizero-1",
		Type: "raspizero",
	}
	m.targets["iot-raspizero-2"] = &model.Target{
		ID:   "iot-raspizero-2",
		Type: "raspizero",
	}

	go startRESTAPI(":8080", m)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, os.Kill)
	<-sig
}
