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
	Git   *Git   `json:"git"`
	Order *Order `json:"order"`
}

func ExecDir(workDir string) string {
	if _, err := os.Stat(fmt.Sprintf("%s/%s", workDir, PackageDir)); err != nil && os.IsNotExist(err) {
		return SourceDir
	}
	return PackageDir
}
