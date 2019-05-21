package source

import (
	"fmt"
	"log"

	copier "github.com/otiai10/copy"
)

type Order string

// Fetch copies artifacts from source into destination order
func (sourceID Order) Fetch(destID string) error {
	log.Printf("Fetching artifacts from %s to %s", sourceID, destID)
	workDir := fmt.Sprintf("%s/%s", OrdersDir, sourceID)
	execDir, found := ExecDir(workDir)
	if !found {
		return fmt.Errorf("%s has no artifacts", sourceID)
	}
	err := copier.Copy(fmt.Sprintf("%s/%s", workDir, execDir), fmt.Sprintf("%s/%s/%s", OrdersDir, destID, execDir))
	if err != nil {
		return err
	}
	return nil
}
