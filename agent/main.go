package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.SetOutput(os.Stdout)
	log.Println("started deployment agent")
	defer log.Println("bye.")

	agent := startAgent()
	defer agent.close()

	subEndpoint, pubEndpoint := endpoints()
	zmqClient, err := startZMQClient(subEndpoint, pubEndpoint, agent.pipe)
	if err != nil {
		log.Fatalf("Error starting ZeroMQ client: %s", err)
	}
	defer zmqClient.close()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, os.Kill)
	<-sig
}

func endpoints() (string, string) {
	prot := "tcp"
	addr := os.Getenv("MANAGER")
	if addr == "" {
		addr = "localhost"
	}
	sub := os.Getenv("SUB")
	if sub == "" {
		sub = "5556"
	}
	pub := os.Getenv("PUB")
	if pub == "" {
		pub = "5557"
	}
	return fmt.Sprintf("%s://%s:%s", prot, addr, sub), fmt.Sprintf("%s://%s:%s", prot, addr, sub)
}
