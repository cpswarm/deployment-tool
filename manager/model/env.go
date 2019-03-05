package model

import "os"

const (
	EnvDebug = "DEBUG" // print debug messages
)

// Env returns the boolean value of the env variable with the given key
func Env(key string) bool {
	return os.Getenv(key) == "1" || os.Getenv(key) == "true" || os.Getenv(key) == "TRUE"
}
