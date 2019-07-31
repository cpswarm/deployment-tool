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
	"code.linksmart.eu/dt/deployment-tool/manager/model"
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

	zmqServer, err := zeromq.StartServer("tcp://*:"+os.Getenv(EnvZeromqPubPort), "tcp://*:"+os.Getenv(EnvZeromqSubPort))
	if err != nil {
		log.Fatalf("Error starting ZeroMQ client: %s", err)
	}
	defer zmqServer.Close()

	zmqConf := model.ZeromqServerInfo{
		PublicKey: zmqServer.PublicKey,
		PubPort:   os.Getenv(EnvZeromqPubPort),
		SubPort:   os.Getenv(EnvZeromqSubPort),
	}

	m, err := startManager(zmqServer.Pipe, zmqConf, os.Getenv(EnvStorageDSN))
	if err != nil {
		log.Fatalf("Error starting manager: %s", err)
	}

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
