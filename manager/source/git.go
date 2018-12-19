package source

import "fmt"

type Git struct {
	// TODO
}

func (git Git) Clone(workDir, orderID string) error {
	return fmt.Errorf("git source loading is not implemented")
}
