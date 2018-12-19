package source

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"log"

	"github.com/mholt/archiver"
)

type Zip string

// Decode base64 encoded zip archive and write it to order directory
func (zip Zip) Store(workDir, orderID string) error {
	log.Println("Storing the base64 encoded archive...")
	data, err := base64.StdEncoding.DecodeString(string(zip))
	if err != nil {
		return err
	}
	log.Printf("Size of data: %d bytes", len(data))
	err = archiver.Zip.Read(bytes.NewBuffer(data), fmt.Sprintf("%s/orders/%s", workDir, orderID))
	if err != nil {
		return err
	}
	return nil
}
