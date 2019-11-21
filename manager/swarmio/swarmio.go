package swarmio

import (
	"crypto/ed25519"
	"io/ioutil"
	"log"
	"os"
)

const (
	EnvCommCert         = "SWARMIO_CERT"
	DefaultCommCertPath = "swarmio.json"
)

type Cert struct {
	PrivateKey []byte `json:"privateKey,omitempty"`
	PublicKey  []byte `json:"publicKey,omitempty"`
	Signature  []byte `json:"signature,omitempty"`
	CA         []byte `json:"ca,omitempty"`
}

func init() {
	if os.Getenv(EnvCommCert) == "" {
		log.Printf("swarmio: cert path not given. Using default path: %s", DefaultCommCertPath)
		os.Setenv(EnvCommCert, DefaultCommCertPath)
	}
}

func CreateKeys(store bool) (public, private []byte, err error) {
	path := os.Getenv(EnvCommCert)

	if _, err := os.Stat(path); os.IsNotExist(err) {
		public, private, err = ed25519.GenerateKey(nil)
		if err != nil {
			return nil, nil, err
		}
		if store {
			err = StoreCert(Cert{
				PrivateKey: private,
				PublicKey:  public,
			})
			if err != nil {
				return nil, nil, err
			}
		}
	} else {
		cert, err := LoadCert()
		if err != nil {
			return nil, nil, err
		}
		private = cert.PrivateKey
		public = cert.PublicKey
		log.Println("swarmio: loaded existing cert.")
	}

	return public, private, nil
}

func StoreCert(cert Cert) error {
	path := os.Getenv(EnvCommCert)

	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0400)
	if err != nil {
		return err
	}
	defer f.Close()

	b, err := EncodeCert(cert)
	if err != nil {
		return err
	}
	if _, err := f.Write(b); err != nil {
		return err
	}

	log.Println("swarmio: Saved certificate:", path)
	return nil
}

func LoadCert() (Cert, error) {
	path := os.Getenv(EnvCommCert)

	b, err := ioutil.ReadFile(path)
	if err != nil {
		return Cert{}, err
	}
	return DecodeCert(b)
}

func SignPublicKey(publicKey []byte) (*Cert, error) {
	cert, err := LoadCert()
	if err != nil {
		return nil, err
	}

	return &Cert{
		Signature: ed25519.Sign(cert.PrivateKey, publicKey),
		CA:        cert.PublicKey,
	}, nil
}
