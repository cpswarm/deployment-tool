package source

import (
	"encoding/base64"
	"fmt"
	"log"

	"code.linksmart.eu/dt/deployment-tool/manager/model"
)

type Zip string // TODO change to []byte?

// Decode base64 encoded zip archive and write it to order directory
func (zip Zip) Store(orderID string) error {
	log.Println("Storing the base64 encoded archive...")
	data, err := base64.StdEncoding.DecodeString(string(zip))
	if err != nil {
		return err
	}
	log.Printf("Size of data: %d bytes", len(data))
	err = model.DecompressFiles(data, fmt.Sprintf("%s/%s/%s", OrdersDir, orderID, SourceDir))
	if err != nil {
		return err
	}
	return nil
}
