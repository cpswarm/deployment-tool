package main

import (
	"flag"
	"log"
	"os"
	"os/signal"

	"code.linksmart.eu/dt/deployment-tool/manager/env"
	"code.linksmart.eu/dt/deployment-tool/manager/zeromq"
)

const (
	// Environment keys
	EnvDisableAuth = "DISABLE_AUTH" // disable authentication completely
	EnvPrivateKey  = "PRIVATE_KEY"  // path to private key of agent
	EnvPublicKey   = "PUBLIC_KEY"   // path to public key of agent
	EnvManagerAddr = "MANAGER_ADDR"
	EnvAuthToken   = "AUTH_TOKEN"
	// Default values
	DefaultEnvFile        = "./.env"       // path to environment variables file
	DefaultStateFile      = "./state.json" // path to agent state file
	DefaultPrivateKeyPath = "./agent.key"
	DefaultPublicKeyPath  = "./agent.pub"
)

var WorkDir = "."

func main() {
	parseFlags()

	log.Println("Started deployment agent")
	defer log.Println("bye.")

	WorkDir, _ = os.Getwd()
	log.Printf("Workdir: %s", WorkDir)

	target, err := loadConf()
	if err != nil {
		log.Fatalf("Error loading config: %s.", err)
	}

	// TODO switch the start order of zmq and agent to facilitate deferred closing in the correct order
	agent, err := startAgent(target, target.ManagerAddr)
	if err != nil {
		log.Fatalf("Error starting agent: %s.", err)
	}

	zmqClient, err := startZMQClient(&target.ZeromqServerConf, target.PublicKey, agent.pipe)
	if err != nil {
		log.Fatalf("Error starting ZeroMQ client: %s.", err)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, os.Kill)
	<-sig

	agent.close()
	zmqClient.close()
}

func init() {
	log.SetOutput(os.Stdout)

	// load env file
	wd, _ := os.Getwd()
	log.Println("Working directory:", wd)

	var logFlags int
	if env.LogTimestamps {
		logFlags = log.LstdFlags
	}
	if env.Verbose {
		logFlags = logFlags | log.Lshortfile
	}
	log.SetFlags(logFlags)
}

func parseFlags() {
	name := flag.String("newkeypair", "", "Generate new Curve keypair with the given name")
	fresh := flag.Bool("fresh", false, "Run after generating new Curve keypair")
	flag.Parse()

	// Generate keypair and exit
	if *name != "" {
		err := zeromq.NewCurveKeypair(*name+".key", *name+".pub")
		if err != nil {
			log.Fatalln("Error creating keypair:", err)
		}
		os.Exit(0)
	}

	// Generate keypair and continue
	if *fresh == true {
		log.Println("Fresh start mode.")
		err := writeNewKeys()
		if err != nil {
			log.Fatalln("Error creating keypair:", err)
		}
	}

}
