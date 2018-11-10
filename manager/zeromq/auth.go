package zeromq

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"

	zmq "github.com/pebbe/zmq4"
)

const (
	DomainAll = "*" // ZAP Domain for access control (https://rfc.zeromq.org/spec:27/ZAP)
	// key files
	PrivateKey     = "manager.key"
	PublicKey      = "manager.pub"
	AuthorizedKeys = "authorized.pubs"
)

func NewCurveKeypair(privateFile, publicFile string) error {
	public, private, err := zmq.NewCurveKeypair()
	if err != nil {
		return fmt.Errorf("error creating keypair: %s", err)
	}

	err = ioutil.WriteFile(privateFile, []byte(private), 0400)
	if err != nil {
		return fmt.Errorf("error writing private key: %s", err)
	}
	fmt.Println("Saved private key to", privateFile)

	err = ioutil.WriteFile(publicFile, []byte(public), 0444)
	if err != nil {
		return fmt.Errorf("error writing public key: %s", err)
	}
	fmt.Println("Saved public key to", publicFile)

	return nil
}

func loadServerKey() (string, error) {
	key, err := ioutil.ReadFile(PrivateKey)
	if err != nil {
		return "", fmt.Errorf("error reading server private key: %s", err)
	}
	fmt.Println("Loaded server key.")
	return string(key), nil
}

func loadClientKeys() error {
	file, err := ioutil.ReadFile(AuthorizedKeys)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("Client keys file %s not found.", AuthorizedKeys)
			return nil
		}
		return fmt.Errorf("error reading client public key: %s", err)
	}

	for i, line := range strings.Split(string(file), "\n") {
		if line == "" { // blank line or EOF
			continue
		}
		parts := strings.Split(line, " ")
		if len(parts) != 2 {
			log.Println("Invalid format in client key file line", i+1)
			continue
		}
		zmq.AuthCurveAdd(DomainAll, parts[1])
		log.Println("Added client key for:", parts[0])
	}

	return nil
}

// TODO add mutex or synchronize it using the pipe channel
func AddClientKey(id, key string) error {
	log.Println("Adding client key for:", id)
	zmq.AuthCurveAdd(DomainAll, key)

	// If the file doesn't exist, create it, or append to the file
	f, err := os.OpenFile(AuthorizedKeys, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0444)
	if err != nil {
		log.Println("Error opening client key file", err)
		return err
	}
	if _, err := f.Write([]byte(fmt.Sprintf("%s %s\n", id, key))); err != nil {
		log.Println("Error writing to client key file:", err)
		return err
	}
	if err := f.Close(); err != nil {
		log.Println("Error closing client key file:", err)
		return err
	}

	return nil
}
