package source

import (
	"fmt"
	"os"
)

const (
	OrdersDir     = "orders" // w/o trailing slash
	SourceDir     = "src"
	SourceArchive = "src.tgz"
	PackageDir    = "pkg"
)

type Source struct {
	Paths *Paths `json:"paths"`
	Zip   *Zip   `json:"zip"`
	Order *Order `json:"order"`
}

func ExecDir(workDir string) (dir string, found bool) {
	if exists(fmt.Sprintf("%s/%s", workDir, PackageDir)) {
		return PackageDir, true
	} else if exists(fmt.Sprintf("%s/%s", workDir, SourceDir)) {
		return SourceDir, true
	}
	return "", false
}

func exists(path string) bool {
	if _, err := os.Stat(path); err != nil && os.IsNotExist(err) {
		return false
	}
	return true
}
