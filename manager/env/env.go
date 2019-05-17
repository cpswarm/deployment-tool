package env

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

const (
	_debug          = "DEBUG"            // print debug messages
	_verbose        = "VERBOSE"          // print extra information e.g. line number)
	_disableLogTime = "DISABLE_LOG_TIME" // disable timestamp in logs
	_envFile        = "./.env"           // path to environment variables file
)

var (
	Debug         = false
	Verbose       = false
	LogTimestamps = true
)

// Env returns the boolean value of the env variable with the given key
func Eval(key string) bool {
	return os.Getenv(key) == "1" || os.Getenv(key) == "true" || os.Getenv(key) == "TRUE"
}

func init() {
	// set env variables from file
	err := godotenv.Load(_envFile)
	if err == nil {
		log.Println("Loaded environment file:", _envFile)
	}

	Debug = Eval(_debug)
	Verbose = Eval(_verbose)
	LogTimestamps = !Eval(_disableLogTime)

	if Debug {
		log.Printf("All environment variables:\n%s", os.Environ())
	} else {
		log.Println(_debug, Debug)
		log.Println(_verbose, Verbose)
		log.Println(_disableLogTime, !LogTimestamps)
	}
}
