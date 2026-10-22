package api

import (
	"fmt"
	"time"

	"ppeelink/models"
)

// trafficSummary 汇总订阅的到期时间。
type trafficSummary struct {
	Upload, Download, Total int64
	Expire                  *time.Time
	Reported                bool
}

func (t *trafficSummary) addExpire(at *time.Time) {
	if at != nil && (t.Expire == nil || at.Before(*t.Expire)) {
		v := *at
		t.Expire = &v
	}
}

// header 生成 Subscription-Userinfo；既无流量也无到期时间时返回空串。
func (t trafficSummary) header() string {
	if !t.Reported && t.Expire == nil {
		return ""
	}
	var expire int64
	if t.Expire != nil {
		expire = t.Expire.Unix()
	}
	return fmt.Sprintf("upload=%d; download=%d; total=%d; expire=%d", t.Upload, t.Download, t.Total, expire)
}

// subscriptionUserinfo 订阅下发时的过期时间头。
func subscriptionUserinfo(sub models.Subcription) string {
	var summary trafficSummary
	summary.addExpire(sub.ExpiresAt)
	return summary.header()
}
