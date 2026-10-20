// This strict native fixture tests the adapter contract; it is not mkcert.
package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	if len(os.Args) == 2 && os.Args[1] == "-version" {
		fmt.Println("v1.4.4")
		return
	}
	args := os.Args[1:]
	dir, _ := os.Getwd()
	if len(args) < 5 || args[0] != "-cert-file" || args[1] != "cert.pem" || args[2] != "-key-file" || args[3] != "key.pem" || os.Getenv("CAROOT") != dir || os.Getenv("TRUST_STORES") != "none" || len(os.Environ()) != 4 {
		panic("unsafe adapter contract")
	}
	if strings.Contains(filepath.Base(os.Args[0]), "fail") {
		os.WriteFile("interrupted", []byte("preserve"), 0600)
		os.Exit(17)
	}
	names := args[4:]
	for _, n := range names {
		if !strings.HasSuffix(n, ".test") || strings.Contains(n, "*") {
			panic("unexpected name")
		}
	}
	if strings.Contains(filepath.Base(os.Args[0]), "wrong") {
		names = []string{"wrong.test"}
	}
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	ca := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Velnix test fixture only"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(24 * time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature}
	caDER, e := x509.CreateCertificate(rand.Reader, ca, ca, &key.PublicKey, key)
	if e != nil {
		panic(e)
	}
	leafKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	leaf := &x509.Certificate{SerialNumber: big.NewInt(2), DNSNames: names, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(12 * time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	leafDER, e := x509.CreateCertificate(rand.Reader, leaf, ca, &leafKey.PublicKey, key)
	if e != nil {
		panic(e)
	}
	write := func(name, kind string, b []byte, mode os.FileMode) {
		if e := os.WriteFile(name, pem.EncodeToMemory(&pem.Block{Type: kind, Bytes: b}), mode); e != nil {
			panic(e)
		}
	}
	caKey, _ := x509.MarshalPKCS8PrivateKey(key)
	private, _ := x509.MarshalPKCS8PrivateKey(leafKey)
	write("rootCA.pem", "CERTIFICATE", caDER, 0644)
	write("rootCA-key.pem", "PRIVATE KEY", caKey, 0600)
	write("cert.pem", "CERTIFICATE", leafDER, 0644)
	write("key.pem", "PRIVATE KEY", private, 0600)
}
