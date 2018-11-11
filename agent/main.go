package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"

	"code.linksmart.eu/dt/deployment-tool/manager/zeromq"
	"github.com/joho/godotenv"
)

const (
	EnvFile   = ".env"
	StateFile = "state.json"
)

func main() {
	if parseFlags() {
		return
	}

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
	return fmt.Sprintf("%s://%s:%s", prot, addr, sub), fmt.Sprintf("%s://%s:%s", prot, addr, pub)
}

func init() {
	log.SetFlags(log.LstdFlags)
	log.SetOutput(os.Stdout)

	// load env file
	wd, _ := os.Getwd()
	log.Println("Working directory:", wd)
	err := godotenv.Load(EnvFile)
	if err == nil {
		log.Println("Loaded environment file:", EnvFile)
	}

	if os.Getenv("DEBUG") != "" {
		log.SetFlags(log.LstdFlags | log.Lshortfile)
	}
}

const (
	PrivateKey = "agent.key"
	PublicKey  = "agent.pub"
)

func parseFlags() bool {
	newKeys := flag.Bool("newkeypair", false, "Generate new Curve keypair")
	flag.Parse()
	if *newKeys {
		err := zeromq.NewCurveKeypair(PrivateKey, PublicKey)
		if err != nil {
			fmt.Println("Error creating keypair:", err)
			os.Exit(1)
		}
		return true
	}
	// nothing is parsed
	return false
}
