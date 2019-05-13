package zeromq

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"

	zmq "github.com/pebbe/zmq4"
)

const (
	DomainAll = "*" // ZAP Domain for access control (https://rfc.zeromq.org/spec:27/ZAP)
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
	encoded := base64.StdEncoding.EncodeToString([]byte(private))
	if _, err := f.Write([]byte(encoded)); err != nil {
		return err
	}
	// write public key
	encoded = base64.StdEncoding.EncodeToString([]byte(public))
	if _, err := f2.Write([]byte(encoded)); err != nil {
		return err
	}
	fmt.Println("Saved key pair:")
	fmt.Printf("\t%s (private key)\n", privateFile)
	fmt.Printf("\t%s (public key) -> %s\n", publicFile, encoded)

	return nil
}

const (
	EnvPrivateKey     = "PRIVATE_KEY"
	EnvPublicKey      = "PUBLIC_KEY"
	EnvAuthorizedKeys = "AUTHORIZED_KEYS"

	DefaultPrivateKeyPath     = "./manager.key"
	DefaultPublicKeyPath      = "./manager.pub"
	DefaultAuthorizedKeysPath = "./authorized_keys"
)

func loadClientKeys() error {
	authorizedKeysPath := os.Getenv(EnvAuthorizedKeys)
	if authorizedKeysPath == "" {
		authorizedKeysPath = DefaultAuthorizedKeysPath
		log.Printf("zeromq: %s not set. Using default path: %s", EnvAuthorizedKeys, DefaultAuthorizedKeysPath)
	}

	file, err := ioutil.ReadFile(authorizedKeysPath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("zeromq: Client keys file %s not found.", authorizedKeysPath)
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
			log.Println("zeromq: Invalid format in authorized keys file line", i+1)
			continue
		}
		decoded, err := base64.StdEncoding.DecodeString(parts[1])
		if err != nil {
			log.Printf("zeromq: Unable to decode client key for %s in authorized keys file line %d", parts[0], i+1)
			continue
		}
		zmq.AuthCurveAdd(DomainAll, string(decoded))
		log.Println("zeromq: Added client key for:", parts[0])
	}

	return nil
}

// TODO add mutex or synchronize it using the pipe channel
func AddClientKey(id, key string) error {
	log.Println("zeromq: Adding client key for:", id)
	zmq.AuthCurveAdd(DomainAll, key)

	authorizedKeysPath := os.Getenv(EnvAuthorizedKeys)
	if authorizedKeysPath == "" {
		authorizedKeysPath = DefaultAuthorizedKeysPath
		log.Printf("zeromq: %s not set. Using default path: %s", EnvAuthorizedKeys, DefaultAuthorizedKeysPath)
	}

	// If the file doesn't exist, create it, or append to the file
	f, err := os.OpenFile(authorizedKeysPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0444)
	if err != nil {
		log.Println("zeromq: Error opening client key file", err)
		return err
	}
	defer f.Close()
	if _, err := f.Write([]byte(fmt.Sprintf("%s %s\n", id, key))); err != nil {
		log.Println("zeromq: Error writing to client key file:", err)
		return err
	}

	return nil
}

func ReadKeyFile(path, defaultPath string) (string, error) {
	if path == "" {
		path = defaultPath
		log.Printf("%s not set. Using default path: %s", path, defaultPath)
	}

	key, err := ioutil.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("error reading file %s: %s", path, err)
	}

	return string(bytes.TrimSpace(key)), nil
}

func DecodeKey(in string) (string, error) {
	b, err := base64.StdEncoding.DecodeString(in)
	if err != nil {
		return "", fmt.Errorf("error decoding string: %s", err)
	}
	return string(b), nil
}
