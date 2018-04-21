package main

import (
	"log"
	"os"
	"os/signal"
)

func main() {
	log.Println("start")

	StartMQTTClient("tcp://localhost:1883")

	handler := make(chan os.Signal, 1)
	signal.Notify(handler, os.Interrupt, os.Kill)
	<-handler
	log.Println("bye.")
}
