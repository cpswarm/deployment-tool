package main

import (
	"log"
	"os"
	"os/signal"
	"path/filepath"
)

func main() {
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

func init() {
	loggingFlags := log.LstdFlags
	if os.Getenv("DEBUG") != "" {
		loggingFlags = log.LstdFlags | log.Lshortfile
	}
	log.SetFlags(loggingFlags)
	log.SetOutput(os.Stdout)

	if os.Getenv("WORKDIR") == "" {
		dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
		if err != nil {
			log.Fatal(err)
		}
		err = os.Setenv("WORKDIR", dir)
		if err != nil {
			log.Fatal(err)
		}
	}
}
