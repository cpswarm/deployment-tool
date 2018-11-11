package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"

	"code.linksmart.eu/dt/deployment-tool/manager/zeromq"
)

func main() {
	if parseFlags() {
		return
	}

	log.Println("started deployment manager")
	defer log.Println("bye.")

	zmqServer, err := zeromq.StartServer("tcp://*:5556", "tcp://*:5557")
	if err != nil {
		log.Fatalf("Error starting ZeroMQ client: %s", err)
	}
	defer zmqServer.Close()

	m, err := startManager(zmqServer.Pipe)
	if err != nil {
		log.Fatal(err)
	}

	go startRESTAPI(":8080", m)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, os.Kill)
	<-sig
}

func init() {
	loggingFlags := log.LstdFlags
	if os.Getenv("DEBUG") != "" {
		loggingFlags = log.LstdFlags | log.Lshortfile
	}
	log.SetFlags(loggingFlags)
	log.SetOutput(os.Stdout)

	if os.Getenv("WORKDIR") == "" {
		dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
		if err != nil {
			log.Fatal(err)
		}
		err = os.Setenv("WORKDIR", dir)
		if err != nil {
			log.Fatal(err)
		}
	}
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
