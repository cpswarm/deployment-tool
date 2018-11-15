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
	// Environment keys
	EnvDebug            = "DEBUG"              // print debug messages
	EnvVerbose          = "VERBOSE"            // print extra information e.g. line number)
	EnvDisableLogTime   = "DISABLE_LOG_TIME"   // disable timestamp in logs
	EnvPrivateKey       = "PRIVATE_KEY"        // path to private key of agent
	EnvPublicKey        = "PUBLIC_KEY"         // path to public key of agent
	EnvManagerPublicKey = "MANAGER_PUBLIC_KEY" // path to public key of manager
	EnvManagerHost      = "MANAGER_HOST"
	EnvManagerSubPort   = "MANAGER_SUB_PORT"
	EnvManagerPubPort   = "MANAGER_PUB_PORT"
	// Default values
	DefaultEnvFile        = "./.env"       // path to environment variables file
	DefaultStateFile      = "./state.json" // path to agent state file
	DefaultPrivateKeyPath = "./agent.key"
	DefaultPublicKeyPath  = "./agent.pub"
	DefaultManagerKeyPath = "./manager.pub"
	DefaultManagerHost    = "localhost"
	DefaultManagerSubPort = "5556"
	DefaultManagerPubPort = "5557"
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
	addr := os.Getenv(EnvManagerHost)
	if addr == "" {
		addr = DefaultManagerHost
		log.Printf("%s not set. Using default: %s", EnvManagerHost, DefaultManagerHost)
	}
	sub := os.Getenv(EnvManagerSubPort)
	if sub == "" {
		sub = DefaultManagerSubPort
		log.Printf("%s not set. Using default: %s", EnvManagerSubPort, DefaultManagerSubPort)
	}
	pub := os.Getenv(EnvManagerPubPort)
	if pub == "" {
		pub = DefaultManagerPubPort
		log.Printf("%s not set. Using default: %s", EnvManagerPubPort, DefaultManagerPubPort)
	}
	return fmt.Sprintf("%s://%s:%s", prot, addr, sub), fmt.Sprintf("%s://%s:%s", prot, addr, pub)
}

var envDebug = false

func init() {
	log.SetOutput(os.Stdout)

	// load env file
	wd, _ := os.Getwd()
	log.Println("Working directory:", wd)
	err := godotenv.Load(DefaultEnvFile)
	if err == nil {
		log.Println("Loaded environment file:", DefaultEnvFile)
	}

	logFlags := log.LstdFlags
	if os.Getenv(EnvDisableLogTime) == "1" || os.Getenv(EnvDisableLogTime) == "true" {
		logFlags = 0
	}

	if os.Getenv(EnvDebug) == "1" || os.Getenv(EnvDebug) == "true" {
		envDebug = true
	}
	if os.Getenv(EnvVerbose) == "1" || os.Getenv(EnvVerbose) == "true" {
		logFlags = logFlags | log.Lshortfile
	}
	log.SetFlags(logFlags)
}

func parseFlags() bool {
	newKeys := flag.Bool("newkeypair", false, "Generate new Curve keypair")
	flag.Parse()
	if *newKeys {
		err := zeromq.NewCurveKeypair("agent.key", "agent.pub")
		if err != nil {
			fmt.Println("Error creating keypair:", err)
			os.Exit(1)
		}
		return true
	}
	// nothing is parsed
	return false
}
