// Copyright (c) 2016 Company 0, LLC.
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package server

// newTLSCertPair returns a new PEM-encoded x.509 certificate pair based on a
import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"os"
	"time"
)

// interfaceAddrs returns a list of the system's network interface addresses.
// It is wrapped here so that we can substitute it for other functions when
// building for systems that do not allow access to net.InterfaceAddrs().
func interfaceAddrs() ([]net.Addr, error) {
	return net.InterfaceAddrs()
}

// 521-bit ECDSA private key.  The machine's local interface addresses and all
// variants of IPv4 and IPv6 localhost are included as valid IP addresses.
func newTLSCertPair(organization string, validUntil time.Time,
	extraHosts []string) (cert, key []byte, err error) {
	now := time.Now()
	if validUntil.Before(now) {
		return nil, nil, errors.New("validUntil would create an " +
			"already-expired certificate")
	}

	priv, err := ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}

	// end of ASN.1 time
	endOfTime := time.Date(2049, 12, 31, 23, 59, 59, 0, time.UTC)
	if validUntil.After(endOfTime) {
		validUntil = endOfTime
	}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate serial "+
			"number: %s", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{organization},
		},
		NotBefore: now.Add(-time.Hour * 24),
		NotAfter:  validUntil,

		KeyUsage: x509.KeyUsageKeyEncipherment |
			x509.KeyUsageDigitalSignature |
			x509.KeyUsageCertSign,
		IsCA:                  true, // so can sign self.
		BasicConstraintsValid: true,
	}

	host, err := os.Hostname()
	if err != nil {
		return nil, nil, err
	}

	// Use maps to prevent adding duplicates.
	ipAddresses := map[string]net.IP{
		"127.0.0.1": net.ParseIP("127.0.0.1"),
		"::1":       net.ParseIP("::1"),
	}
	dnsNames := map[string]bool{
		host:        true,
		"localhost": true,
	}

	addrs, err := interfaceAddrs()
	if err != nil {
		return nil, nil, err
	}
	for _, a := range addrs {
		ip, _, err := net.ParseCIDR(a.String())
		if err == nil {
			ipAddresses[ip.String()] = ip
		}
	}

	for _, hostStr := range extraHosts {
		host, _, err := net.SplitHostPort(hostStr)
		if err != nil {
			host = hostStr
		}
		if ip := net.ParseIP(host); ip != nil {
			ipAddresses[ip.String()] = ip
		} else {
			dnsNames[host] = true
		}
	}

	template.DNSNames = make([]string, 0, len(dnsNames))
	for dnsName := range dnsNames {
		template.DNSNames = append(template.DNSNames, dnsName)
	}
	template.IPAddresses = make([]net.IP, 0, len(ipAddresses))
	for _, ip := range ipAddresses {
		template.IPAddresses = append(template.IPAddresses, ip)
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template,
		&template, &priv.PublicKey, priv)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create "+
			"certificate: %v", err)
	}

	certBuf := &bytes.Buffer{}
	err = pem.Encode(certBuf, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: derBytes,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to encode certificate: %v",
			err)
	}

	keybytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal private key: %v",
			err)
	}

	keyBuf := &bytes.Buffer{}
	err = pem.Encode(keyBuf, &pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: keybytes,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to encode private key: %v",
			err)
	}

	return certBuf.Bytes(), keyBuf.Bytes(), nil
}
