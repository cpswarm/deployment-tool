package swarmio

import (
	"encoding/json"
)

func EncodeCert(cert Cert) ([]byte, error) {
	return json.MarshalIndent(cert, "", "\t")
}

func DecodeCert(b []byte) (Cert, error) {
	var cert Cert
	err := json.Unmarshal(b, &cert)
	if err != nil {
		return Cert{}, err
	}
	return cert, nil
}
