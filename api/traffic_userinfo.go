package api

import (
	"fmt"
	"time"

	"ppeelink/models"
)

// trafficSummary 汇总若干上游最近一次上报的流量与最早到期时间。
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

// sourceTraffic 订阅上游最近一次上报的流量。
func sourceTraffic(sub models.Subcription) trafficSummary {
	var out trafficSummary
	if sub.SourceTrafficAt == nil {
		return out
	}
	out.Reported = true
	out.Upload, out.Download, out.Total = sub.SourceUpload, sub.SourceDownload, sub.SourceTotal
	out.addExpire(sub.SourceExpire)
	return out
}

// subscriptionUserinfo 订阅下发时的流量头：上游流量，到期取上游与订阅中较早者。
func subscriptionUserinfo(sub models.Subcription) string {
	summary := sourceTraffic(sub)
	summary.addExpire(sub.ExpiresAt)
	return summary.header()
}

// collectionUserinfo 合集汇总各成员订阅的上游流量。
func collectionUserinfo(col models.Collection) string {
	var summary trafficSummary
	for _, id := range parseMemberIDs(col.MemberIDs) {
		var sub models.Subcription
		if models.DB.First(&sub, id).Error != nil {
			continue
		}
		part := sourceTraffic(sub)
		if part.Reported {
			summary.Reported = true
			summary.Upload += part.Upload
			summary.Download += part.Download
			summary.Total += part.Total
		}
		summary.addExpire(part.Expire)
	}
	summary.addExpire(col.ExpiresAt)
	return summary.header()
}
