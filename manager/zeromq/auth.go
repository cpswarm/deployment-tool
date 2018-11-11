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
	AuthorizedKeys = "authorized_keys"
)

func NewCurveKeypair(privateFile, publicFile string) error {
	public, private, err := zmq.NewCurveKeypair()
	if err != nil {
		return fmt.Errorf("error creating keypair: %s", err)
	}

	// open both files
	f, err := os.OpenFile(privateFile, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0400)
	if err != nil {
		return err
	}
	defer f.Close()
	f2, err := os.OpenFile(publicFile, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0444)
	if err != nil {
		return err
	}
	defer f2.Close()

	// write private key
	if _, err := f.Write([]byte(private)); err != nil {
		return err
	}
	// write public key
	if _, err := f2.Write([]byte(public)); err != nil {
		return err
	}
	fmt.Println("Saved key pair:")
	fmt.Printf("\t%s (private key)\n", privateFile)
	fmt.Printf("\t%s (public key) -> %s\n", publicFile, public)

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
	defer f.Close()
	if _, err := f.Write([]byte(fmt.Sprintf("%s %s\n", id, key))); err != nil {
		log.Println("Error writing to client key file:", err)
		return err
	}

	return nil
}
