package main

import (
	"log"
	"os"
	"os/signal"
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

	m, err := startManager(zmqClient.pipe)
	if err != nil {
		log.Fatal(err)
	}

	go startRESTAPI(":8080", m)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, os.Kill)
	<-sig
}
