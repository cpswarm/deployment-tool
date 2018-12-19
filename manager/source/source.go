package source

type Source struct {
	Paths *Paths `json:"paths"`
	Zip   *Zip   `json:"zip"`
	Git   *Git   `json:"git"`
}
