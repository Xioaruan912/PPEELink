package api

import (
	"fmt"
	"gorm.io/gorm"
	"ppeelink/certpin"
	"ppeelink/models"
	"ppeelink/xraycore"
)

func applyCertificateReports(tx *gorm.DB, serverID uint, report xraycore.Report) (bool, error) {
	if len(report.Certificates) > 128 {
		return false, fmt.Errorf("证书回报过多")
	}
	changed := false
	for identity, public := range report.Certificates {
		var in models.ProxyInbound
		if err := tx.Where("server_id = ? AND tls_mode IN ? AND tls_auto_id = ?", serverID, []string{"auto", "self-signed"}, identity).First(&in).Error; err == gorm.ErrRecordNotFound {
			continue
		} else if err != nil {
			return false, err
		}
		cert, pin, err := certpin.Parse(public)
		if err != nil {
			return false, err
		}
		if err = cert.VerifyHostname(xraycore.AutoTLSServerName); err != nil {
			return false, err
		}
		if err = cert.CheckSignatureFrom(cert); err != nil {
			return false, fmt.Errorf("自动证书签名无效")
		}
		if in.TLSFingerprint != "" && in.TLSFingerprint != pin {
			return false, fmt.Errorf("自动证书指纹已变化，拒绝覆盖已绑定身份")
		}
		if in.TLSFingerprint == "" {
			if err = tx.Model(&in).Updates(map[string]any{"tls_certificate": public, "tls_fingerprint": pin}).Error; err != nil {
				return false, err
			}
			changed = true
		}
	}
	return changed, nil
}
