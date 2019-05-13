package model

import (
	"bytes"
	"fmt"
	"log"

	"code.linksmart.eu/dt/deployment-tool/manager/env"
	"github.com/mholt/archiver"
)

// CompressFiles reads from given path and compresses in memory
func CompressFiles(paths ...string) ([]byte, error) {
	if env.Debug {
		log.Printf("Compressing: %v", paths)
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("no path provided")
	}
	var b bytes.Buffer
	err := archiver.Zip.Write(&b, paths)
	if err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// DecompressFiles decompresses from memory and writes to given directory
func DecompressFiles(b []byte, dir string) error {
	if env.Debug {
		log.Printf("Decompressing %d bytes to %s", len(b), dir)
	}
	return archiver.Zip.Read(bytes.NewBuffer(b), dir)
}
