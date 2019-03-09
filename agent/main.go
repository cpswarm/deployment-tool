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
	EnvDebug               = "DEBUG"                  // print debug messages
	EnvVerbose             = "VERBOSE"                // print extra information e.g. line number)
	EnvDisableLogTime      = "DISABLE_LOG_TIME"       // disable timestamp in logs
	EnvDisableAuth         = "DISABLE_AUTH"           // disable authentication completely
	EnvPrivateKey          = "PRIVATE_KEY"            // path to private key of agent
	EnvPublicKey           = "PUBLIC_KEY"             // path to public key of agent
	EnvManagerPublicKey    = "MANAGER_PUBLIC_KEY"     // path to public key of manager
	EnvManagerPublicKeyStr = "MANAGER_PUBLIC_KEY_STR" // public key of manager (overrides file)
	EnvManagerHost         = "MANAGER_HOST"
	EnvManagerSubPort      = "MANAGER_SUB_PORT"
	EnvManagerPubPort      = "MANAGER_PUB_PORT"
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

var WorkDir = "."

func main() {
	parseFlags()

	log.Println("Started deployment agent")
	defer log.Println("bye.")

	WorkDir, _ = os.Getwd()
	log.Printf("Workdir: %s", WorkDir)

	agent, err := startAgent()
	if err != nil {
		log.Fatalf("Error starting agent: %s", err)
	}
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

func init() {
	log.SetOutput(os.Stdout)
	log.SetFlags(0)

	// load env file
	wd, _ := os.Getwd()
	log.Println("Working directory:", wd)
	err := godotenv.Load(DefaultEnvFile)
	if err == nil {
		log.Println("Loaded environment file:", DefaultEnvFile)
	}

	logFlags := log.LstdFlags
	if evalEnv(EnvDisableLogTime) {
		logFlags = 0
	}
	if evalEnv(EnvVerbose) {
		logFlags = logFlags | log.Lshortfile
	}
	log.SetFlags(logFlags)
}

// evalEnv returns the boolean value of the env variable with the given key
func evalEnv(key string) bool {
	return os.Getenv(key) == "1" || os.Getenv(key) == "true" || os.Getenv(key) == "TRUE"
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
