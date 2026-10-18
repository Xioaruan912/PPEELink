package api

import (
	"fmt"
	"time"

	"ppeelink/models"
)

// trafficSummary 汇总若干机场最近一次上报的流量与最早到期时间。
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

// subscriptionGroupNames 订阅直接引用的分组，加上其固定节点所属的分组。
// 机场同步时节点落在与机场同名的分组中，因此分组名即机场名。
func subscriptionGroupNames(subID int) map[string]bool {
	names := map[string]bool{}
	var sub models.Subcription
	if err := models.DB.Preload("GroupRefs").Preload("Nodes.GroupNodes").First(&sub, subID).Error; err != nil {
		return names
	}
	for _, g := range sub.GroupRefs {
		names[g.Name] = true
	}
	for _, n := range sub.Nodes {
		for _, g := range n.GroupNodes {
			names[g.Name] = true
		}
	}
	return names
}

// airportTraffic 汇总名称落在 names 中且上报过流量的机场。
func airportTraffic(names map[string]bool) trafficSummary {
	var out trafficSummary
	if len(names) == 0 {
		return out
	}
	list := make([]string, 0, len(names))
	for n := range names {
		list = append(list, n)
	}
	var airports []models.Airport
	if err := models.DB.Where("name IN ? AND traffic_updated_at IS NOT NULL", list).Find(&airports).Error; err != nil {
		return out
	}
	for _, a := range airports {
		out.Reported = true
		out.Upload += a.TrafficUpload
		out.Download += a.TrafficDownload
		out.Total += a.TrafficTotal
		out.addExpire(a.TrafficExpire)
	}
	return out
}

// subscriptionUserinfo 订阅下发时的流量头：所用机场的流量之和，到期取机场与订阅中较早者。
func subscriptionUserinfo(sub models.Subcription) string {
	summary := airportTraffic(subscriptionGroupNames(sub.ID))
	summary.addExpire(sub.ExpiresAt)
	return summary.header()
}

// collectionUserinfo 合集按成员订阅所用机场去重后汇总。
func collectionUserinfo(col models.Collection) string {
	names := map[string]bool{}
	for _, id := range parseMemberIDs(col.MemberIDs) {
		for n := range subscriptionGroupNames(id) {
			names[n] = true
		}
	}
	summary := airportTraffic(names)
	summary.addExpire(col.ExpiresAt)
	return summary.header()
}
