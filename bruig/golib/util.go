package golib

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
)

func fingerprintDER(c *x509.Certificate) string {
	d := sha256.New()
	d.Write(c.Raw)
	digest := d.Sum(nil)
	return hex.EncodeToString(digest)
}
