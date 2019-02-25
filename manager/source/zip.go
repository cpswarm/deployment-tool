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
func (zip Zip) Store(orderID string) error {
	log.Println("zip: Storing the base64 encoded archive...")
	data, err := base64.StdEncoding.DecodeString(string(zip))
	if err != nil {
		return err
	}
	log.Printf("zip: Size of data: %d bytes", len(data))
	err = archiver.Zip.Read(bytes.NewBuffer(data), fmt.Sprintf("%s/%s/%s", OrdersDir, orderID, SourceDir))
	if err != nil {
		return err
	}
	return nil
}
