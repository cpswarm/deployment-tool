package source

import (
	"fmt"
	"log"

	copier "github.com/otiai10/copy"
)

type Order string

// fetches files from a previous order into workdir/id/<name>
func (order Order) Fetch(orderID string) error {
	prevOrder := fmt.Sprintf("%s/%s/%s", OrdersDir, order, PackageDir)
	log.Println("Copying from prev order...")
	err := copier.Copy(prevOrder, fmt.Sprintf("%s/%s/%s", OrdersDir, orderID, SourceDir))
	if err != nil {
		return err
	}
	return nil
}
