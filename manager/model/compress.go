package model

import (
	"bytes"
	"log"

	"github.com/mholt/archiver"
)

// CompressFiles reads from given path and compresses in memory
func CompressFiles(paths ...string) ([]byte, error) {
	if len(paths) == 0 {
		log.Panicf("no path provided")
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
	return archiver.TarGz.Read(bytes.NewBuffer(b), dir)
}
