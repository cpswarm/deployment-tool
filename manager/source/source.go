package source

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
}
