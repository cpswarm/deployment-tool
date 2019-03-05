package model

import (
	"bytes"
	"fmt"
	"log"

	"github.com/mholt/archiver"
)

// CompressFiles reads from given path and compresses in memory
func CompressFiles(paths ...string) ([]byte, error) {
	if Env(EnvDebug) {
		log.Printf("Compressing: %v", paths)
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("no path provided")
	}
	var b bytes.Buffer
	err := archiver.TarGz.Write(&b, paths)
	if err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// DecompressFiles decompresses from memory and writes to given directory
func DecompressFiles(b []byte, dir string) error {
	if Env(EnvDebug) {
		log.Printf("Decompressing %d bytes to %s", len(b), dir)
	}
	return archiver.TarGz.Read(bytes.NewBuffer(b), dir)
}
