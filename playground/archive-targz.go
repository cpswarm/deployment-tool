package main

import (
	"bytes"
	"fmt"
	"log"
	"time"

	"github.com/mholt/archiver"
)

func main() {
	var b bytes.Buffer
	err := archiver.TarGz.Write(&b, []string{"zeromq_samples"})
	if err != nil {
		log.Fatal(err)
	}

	log.Println(b.Bytes())

	err = archiver.TarGz.Read(&b, fmt.Sprintf("archive-%d", time.Now().Unix()))
	if err != nil {
		log.Fatal(err)
	}
}
