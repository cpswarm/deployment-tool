package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"

	"code.linksmart.eu/dt/deployment-tool/manager/env"
	"code.linksmart.eu/dt/deployment-tool/manager/storage"
	"code.linksmart.eu/dt/deployment-tool/manager/zeromq"
)

const (
	// Environment keys
	EnvWorkdir        = "WORKDIR"         // work directory of the manager
	EnvStorageDSN     = "STORAGE_DSN"     // Storage DSN i.e. Elasticsearch's URL
	EnvZeromqPubPort  = "ZEROMQ_PUB_PORT" // Changes are not propagated to existing agents
	EnvZeromqSubPort  = "ZEROMQ_SUB_PORT" // Changes are not propagated to existing agents
	EnvHTTPServerPort = "HTTP_SERVER_PORT"
	// Defaults
	DefaultStorageDSN     = "http://localhost:9200"
	DefaultZeromqPubPort  = "5556"
	DefaultZeromqSubPort  = "5557"
	DefaultHTTPServerPort = "8080"
)

func main() {
	if parseFlags() {
		return
	}

	log.Println("STARTED DEPLOYMENT MANAGER")
	defer log.Println("bye.")

	storageClient, err := storage.StartElasticStorage(os.Getenv(EnvStorageDSN))
	if err != nil {
		log.Fatalf("Error starting elastic client: %s", err)
	}
	keys, err := storageClient.GetTargetKeys()
	if err != nil {
		log.Fatalf("Error reading public keys from database: %s", err)
	}

	zmqServer, err := zeromq.SetupServer(os.Getenv(EnvZeromqPubPort), os.Getenv(EnvZeromqSubPort), keys)
	if err != nil {
		log.Fatalf("Error starting ZeroMQ client: %s", err)
	}

	m, err := startManager(zmqServer.Pipe, zmqServer.Conf(), storageClient)
	if err != nil {
		log.Fatalf("Error starting manager: %s", err)
	}

	err = zmqServer.Start()
	if err != nil {
		log.Fatalf("Error starting zeromq server: %s", err)
	}
	defer zmqServer.Close()

	go startRESTAPI(":"+os.Getenv(EnvHTTPServerPort), m)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, os.Kill)
	<-sig
}

var (
	WorkDir string
)

func init() {
	log.SetOutput(os.Stdout)

	var logFlags int
	if env.LogTimestamps {
		logFlags = log.LstdFlags
	}
	if env.Verbose {
		logFlags = logFlags | log.Lshortfile
	}
	log.SetFlags(logFlags)

	WorkDir = os.Getenv(EnvWorkdir)
	if WorkDir == "" {
		dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
		if err != nil {
			log.Fatal(err)
		}
		WorkDir = dir
	}

	if os.Getenv(EnvStorageDSN) == "" {
		os.Setenv(EnvStorageDSN, DefaultStorageDSN)
	}
	if os.Getenv(EnvZeromqPubPort) == "" {
		os.Setenv(EnvZeromqPubPort, DefaultZeromqPubPort)
	}
	if os.Getenv(EnvZeromqSubPort) == "" {
		os.Setenv(EnvZeromqSubPort, DefaultZeromqSubPort)
	}
	if os.Getenv(EnvHTTPServerPort) == "" {
		os.Setenv(EnvHTTPServerPort, DefaultHTTPServerPort)
	}
}

func parseFlags() bool {
	name := flag.String("newkeypair", "", "Generate new Curve keypair with the given name")
	flag.Parse()
	if *name != "" {
		err := zeromq.WriteCurveKeypair(*name+".key", *name+".pub")
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
