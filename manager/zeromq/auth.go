package zeromq

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	zmq "github.com/pebbe/zmq4"
)

const (
	DomainAll = "*" // ZAP Domain for access control (https://rfc.zeromq.org/spec:27/ZAP)
)

func WriteCurveKeypair(privateFile, publicFile string) error {
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

func ReadKeyFile(path, defaultPath string) (string, error) {
	if path == "" {
		log.Printf("ReadKeyFile: path not given. Using default path: %s", defaultPath)
		path = defaultPath
	}

	key, err := ioutil.ReadFile(path)
	if err != nil {
		return "", err
	}

	return string(bytes.TrimSpace(key)), nil
}

func DecodeKey(in string) (string, error) {
	b, err := base64.StdEncoding.DecodeString(in)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
