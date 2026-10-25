package xraycore

import (
	"crypto/tls"
	"crypto/x509"
	"os"
	"path/filepath"
	"testing"
)

func TestAutoTLSIdentityPersistsAndRejectsCorruption(t *testing.T) {
	dir := t.TempDir()
	id := "0123456789abcdef0123456789abcdef"
	cert, key, pub, err := ensureAutoTLS(dir, id)
	if err != nil {
		t.Fatal(err)
	}
	pair, err := tls.LoadX509KeyPair(cert, key)
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	if err = leaf.VerifyHostname(AutoTLSServerName); err != nil {
		t.Fatal(err)
	}
	_, _, again, err := ensureAutoTLS(dir, id)
	if err != nil || again != pub {
		t.Fatal("certificate changed on restart")
	}
	if err = os.WriteFile(filepath.Join(dir, "tls", id, "identity.json"), []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err = ensureAutoTLS(dir, id); err == nil {
		t.Fatal("corrupt identity silently replaced")
	}
	if _, _, _, err = ensureAutoTLS(dir, "../../escape"); err == nil {
		t.Fatal("unsafe identity accepted")
	}
}
