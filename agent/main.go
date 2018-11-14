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
	EnvManager        = "MANAGER"
	EnvManagerSub     = "MANAGER_SUB"
	EnvManagerPub     = "MANAGER_PUB"
	EnvDebug          = "DEBUG"
	EnvVerbose        = "VERBOSE"
	EnvDisableLogTime = "DISABLE_LOG_TIME"

	DefaultEnvFile    = "./.env"
	DefaultStateFile  = "./state.json"
	DefaultManager    = "localhost"
	DefaultManagerSub = "5556"
	DefaultManagerPub = "5557"
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
	addr := os.Getenv(EnvManager)
	if addr == "" {
		addr = DefaultManager
		log.Printf("%s not set. Using default: %s", EnvManager, DefaultManager)
	}
	sub := os.Getenv(EnvManagerSub)
	if sub == "" {
		sub = DefaultManagerSub
		log.Printf("%s not set. Using default: %s", EnvManagerSub, DefaultManagerSub)
	}
	pub := os.Getenv(EnvManagerPub)
	if pub == "" {
		pub = DefaultManagerPub
		log.Printf("%s not set. Using default: %s", EnvManagerPub, DefaultManagerPub)
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
