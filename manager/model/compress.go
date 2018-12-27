package model

import (
	"bytes"
	"fmt"

	"github.com/mholt/archiver"
)

// CompressFiles reads from given path and compresses in memory
func CompressFiles(workDir string, paths ...string) ([]byte, error) {
	// make it relative to work directory
	for i, path := range paths {
		paths[i] = fmt.Sprintf("%s/%s", workDir, path)
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
