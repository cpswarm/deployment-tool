package source

import (
	"fmt"
	"log"

	copier "github.com/otiai10/copy"
)

type Order string

// Fetch copies artifacts from source into destination order
func (sourceID Order) Fetch(destID string) error {
	prevOrder := fmt.Sprintf("%s/%s/%s", OrdersDir, sourceID, PackageDir)
	log.Printf("Fetching artifacts from %s to %s", sourceID, destID)
	err := copier.Copy(prevOrder, fmt.Sprintf("%s/%s/%s", OrdersDir, destID, PackageDir))
	if err != nil {
		return err
	}
	return nil
}
