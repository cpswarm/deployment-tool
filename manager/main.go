package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"

	"code.linksmart.eu/dt/deployment-tool/manager/zeromq"
)

const (
	// Environment keys
	EnvDebug      = "DEBUG"       // print debug messages
	EnvVerbose    = "VERBOSE"     // print extra information e.g. line number)
	EnvWorkdir    = "WORKDIR"     // work directory of the manager
	EnvStorageDSN = "STORAGE_DSN" // Storage DSN i.e. Elasticsearch's URL
)

func main() {
	if parseFlags() {
		return
	}

	log.Println("Started deployment manager")
	defer log.Println("bye.")

	zmqServer, err := zeromq.StartServer("tcp://*:5556", "tcp://*:5557")
	if err != nil {
		log.Fatalf("Error starting ZeroMQ client: %s", err)
	}
	defer zmqServer.Close()

	m, err := startManager(zmqServer.Pipe, zmqServer.PublicKey, os.Getenv(EnvStorageDSN))
	if err != nil {
		log.Fatal(err)
	}

	go startRESTAPI(":8080", m)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, os.Kill)
	<-sig
}

var (
	WorkDir string
)

func init() {
	loggingFlags := log.LstdFlags
	if evalEnv(EnvVerbose) {
		log.SetFlags(log.LstdFlags | log.Lshortfile)
	}
	log.SetFlags(loggingFlags)
	log.SetOutput(os.Stdout)

	WorkDir = os.Getenv(EnvWorkdir)
	if WorkDir == "" {
		dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
		if err != nil {
			log.Fatal(err)
		}
		WorkDir = dir
	}

	if os.Getenv(EnvStorageDSN) == "" {
		os.Setenv(EnvStorageDSN, "http://localhost:9200")
	}
}

// evalEnv returns the boolean value of the env variable with the given key
func evalEnv(key string) bool {
	return os.Getenv(key) == "1" || os.Getenv(key) == "true" || os.Getenv(key) == "TRUE"
}

func parseFlags() bool {
	name := flag.String("newkeypair", "", "Generate new Curve keypair with the given name")
	flag.Parse()
	if *name != "" {
		err := zeromq.NewCurveKeypair(*name+".key", *name+".pub")
		if err != nil {
			fmt.Println("Error creating keypair:", err)
			os.Exit(1)
		}
		return true
	}
	// nothing is parsed
	return false
}

func recovery() {
	if r := recover(); r != nil {
		log.Printf("PANIC: %v\n%s", r, debug.Stack())
	}
}
