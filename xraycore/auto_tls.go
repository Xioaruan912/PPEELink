package xraycore

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"ppeelink/certpin"
	"regexp"
	"time"
)

const AutoTLSServerName = "hy2.subl.invalid"

var autoTLSIdentity = regexp.MustCompile(`^[a-f0-9]{32}$`)

type tlsBundle struct {
	Certificate string `json:"certificate"`
	Key         string `json:"key"`
}

func privateWrite(path string, data []byte) error {
	f, e := os.CreateTemp(filepath.Dir(path), ".tls-*")
	if e != nil {
		return e
	}
	name := f.Name()
	defer os.Remove(name)
	if e = f.Chmod(0600); e == nil {
		_, e = f.Write(data)
	}
	if e == nil {
		e = f.Sync()
	}
	closeErr := f.Close()
	if e != nil {
		return e
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(name, path)
}
func ensureAutoTLS(root, id string) (certPath, keyPath, public string, err error) {
	if !autoTLSIdentity.MatchString(id) {
		err = fmt.Errorf("自动证书标识无效")
		return
	}
	dir := filepath.Join(root, "tls", id)
	if err = os.MkdirAll(dir, 0700); err != nil {
		return
	}
	bundlePath := filepath.Join(dir, "identity.json")
	var bundle tlsBundle
	data, e := os.ReadFile(bundlePath)
	if os.IsNotExist(e) {
		key, e := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if e != nil {
			err = e
			return
		}
		serial, e := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
		if e != nil {
			err = e
			return
		}
		now := time.Now()
		template := &x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: AutoTLSServerName}, DNSNames: []string{AutoTLSServerName}, NotBefore: now.Add(-5 * time.Minute), NotAfter: now.AddDate(5, 0, 0), KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, BasicConstraintsValid: true, IsCA: true}
		der, e := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
		if e != nil {
			err = e
			return
		}
		keyDER, e := x509.MarshalPKCS8PrivateKey(key)
		if e != nil {
			err = e
			return
		}
		bundle = tlsBundle{string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})), string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}))}
		data, _ = json.Marshal(bundle)
		if err = privateWrite(bundlePath, data); err != nil {
			return
		}
	} else if e != nil {
		err = e
		return
	} else if len(data) > 16384 || json.Unmarshal(data, &bundle) != nil {
		err = fmt.Errorf("自动证书存储损坏，拒绝更换指纹")
		return
	}
	if _, err = tls.X509KeyPair([]byte(bundle.Certificate), []byte(bundle.Key)); err != nil {
		return
	}
	c, _, e := certpin.Parse(bundle.Certificate)
	if e != nil {
		err = e
		return
	}
	if err = c.VerifyHostname(AutoTLSServerName); err != nil {
		return
	}
	certPath, keyPath = filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem")
	if err = privateWrite(certPath, []byte(bundle.Certificate)); err != nil {
		return
	}
	if err = privateWrite(keyPath, []byte(bundle.Key)); err != nil {
		return
	}
	public = bundle.Certificate
	return
}
func prepareAutoTLS(dir string, spec Spec) (Spec, map[string]string, error) {
	spec.Inbounds = append([]InboundSpec(nil), spec.Inbounds...)
	certs := map[string]string{}
	for i := range spec.Inbounds {
		in := &spec.Inbounds[i]
		if in.TLSAutoID == "" {
			continue
		}
		if in.Template != TemplateHysteria2 {
			return spec, nil, fmt.Errorf("自动证书仅支持 Hysteria2")
		}
		cert, key, public, e := ensureAutoTLS(dir, in.TLSAutoID)
		if e != nil {
			return spec, nil, e
		}
		in.TLSCertFile, in.TLSKeyFile, in.SNI = cert, key, AutoTLSServerName
		certs[in.TLSAutoID] = public
	}
	return spec, certs, nil
}
func (r *Runner) CertificateReports() map[string]string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := map[string]string{}
	for id, cert := range r.certificates {
		out[id] = cert
	}
	return out
}
