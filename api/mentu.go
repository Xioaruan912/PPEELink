package api

import (
	"github.com/gin-gonic/gin"
)

type Meta struct {
	Title     string   `json:"title"`
	Icon      string   `json:"icon"`
	Hidden    bool     `json:"hidden"`
	Roles     []string `json:"roles"`
	KeepAlive bool     `json:"keepAlive,omitempty"`
}

type Child struct {
	Path      string `json:"path"`
	Component string `json:"component"`
	Name      string `json:"name"`
	Meta      Meta   `json:"meta"`
}

type Menu struct {
	Path      string  `json:"path"`
	Component string  `json:"component"`
	Redirect  string  `json:"redirect"`
	Name      string  `json:"name"`
	Meta      Meta    `json:"meta"`
	Children  []Child `json:"children"`
}

func GetMenus(c *gin.Context) {
	menus := []Menu{
		// 普通用户：我的订阅
		{
			Path:      "/my",
			Component: "Layout",
			Redirect:  "/my/subscriptions",
			Name:      "my",
			Meta:      Meta{Title: "我的", Icon: "link", Roles: []string{"USER"}},
			Children: []Child{
				{
					Path:      "subscriptions",
					Component: "my/index",
					Name:      "MySubscriptions",
					Meta:      Meta{Title: "我的订阅", Icon: "link", Roles: []string{"USER"}},
				},
			},
		},
		// 服务器监控
		{
			Path:      "/monitor",
			Component: "Layout",
			Redirect:  "/monitor/index",
			Name:      "monitor",
			Meta:      Meta{Title: "monitor", Icon: "monitor", Roles: []string{"ADMIN"}},
			Children: []Child{
				{
					Path:      "index",
					Component: "monitor/index",
					Name:      "Monitor",
					Meta:      Meta{Title: "monitor", Icon: "monitor", Roles: []string{"ADMIN"}, KeepAlive: true},
				},
				{
					Path:      "website",
					Component: "monitor/website",
					Name:      "MonitorWebsite",
					Meta:      Meta{Title: "监控网站", Icon: "monitor", Roles: []string{"ADMIN"}, KeepAlive: true},
				},
				{Path: "workspace", Component: "monitor/ServerWorkspace", Name: "ServerWorkspace", Meta: Meta{Title: "服务器工作台", Roles: []string{"ADMIN"}, Hidden: true}},
			},
		},
		{
			Path: "/network", Component: "Layout", Redirect: "/network/traceroute", Name: "network",
			Meta: Meta{Title: "网络工具", Icon: "monitor", Roles: []string{"ADMIN"}},
			Children: []Child{
				{Path: "traceroute", Component: "network/Traceroute", Name: "NetworkTraceroute", Meta: Meta{Title: "多地路由追踪", Roles: []string{"ADMIN"}, KeepAlive: true}},
			},
		},
		// 订阅管理
		{
			Path:      "/subcription",
			Component: "Layout",
			Redirect:  "/subcription/subs",
			Name:      "subcription",
			Meta: Meta{
				Title:  "subcription",
				Icon:   "client",
				Hidden: false,
				Roles:  []string{"ADMIN"},
			},
			Children: []Child{
				{
					Path:      "subs",
					Component: "subcription/subs",
					Name:      "Subs",
					Meta: Meta{
						Title:     "sublist",
						Icon:      "link",
						Hidden:    false,
						Roles:     []string{"ADMIN"},
						KeepAlive: true,
					},
				},
				{
					Path:      "nodes",
					Component: "subcription/nodes",
					Name:      "Nodes",
					Meta: Meta{
						Title:     "nodelist",
						Icon:      "publish",
						Hidden:    false,
						Roles:     []string{"ADMIN"},
						KeepAlive: true,
					},
				},
				{
					Path:      "servers",
					Component: "subcription/servers",
					Name:      "ProxyServers",
					Meta:      Meta{Title: "自建节点", Icon: "monitor", Roles: []string{"ADMIN"}, KeepAlive: true},
				},
			},
		},
		// 测试：测速、解锁、TCP 合并为一个页面
		{
			Path:      "/test",
			Component: "Layout",
			Redirect:  "/test/index",
			Name:      "test",
			Meta:      Meta{Title: "test", Icon: "monitor", Roles: []string{"ADMIN"}},
			Children: []Child{
				{
					Path:      "index",
					Component: "test/index",
					Name:      "NodeTest",
					Meta:      Meta{Title: "节点测试", Icon: "monitor", Roles: []string{"ADMIN"}, KeepAlive: true},
				},
			},
		},
		// 模板管理
		{
			Path:      "/template",
			Component: "Layout",
			Redirect:  "/template/list",
			Name:      "template",
			Meta: Meta{
				Title:  "template",
				Icon:   "document",
				Hidden: false,
				Roles:  []string{"ADMIN"},
			},
			Children: []Child{
				{
					Path:      "list",
					Component: "template/list",
					Name:      "TemplateList",
					Meta: Meta{
						Title:     "templatelist",
						Icon:      "document",
						Hidden:    false,
						Roles:     []string{"ADMIN"},
						KeepAlive: true,
					},
				},
				{
					Path:      "builder",
					Component: "template/builder",
					Name:      "TemplateBuilder",
					Meta: Meta{
						Title:     "templatebuilder",
						Icon:      "edit",
						Hidden:    false,
						Roles:     []string{"ADMIN"},
						KeepAlive: true,
					},
				},
				{
					Path:      "rules",
					Component: "template/rules-stable",
					Name:      "RuleCenter",
					Meta: Meta{
						Title:     "rulecenter",
						Icon:      "table",
						Hidden:    false,
						Roles:     []string{"ADMIN"},
						KeepAlive: true,
					},
				},
			},
		},
		// 系统：用户、设置；修改密码对所有账户开放但不进菜单
		{
			Path:      "/system",
			Component: "Layout",
			Redirect:  "/system/users",
			Name:      "system",
			Meta:      Meta{Title: "系统", Icon: "system", Roles: []string{"ADMIN", "USER"}},
			Children: []Child{
				{
					Path:      "users",
					Component: "system/user/list",
					Name:      "UserList",
					Meta:      Meta{Title: "用户管理", Icon: "user", Roles: []string{"ADMIN"}, KeepAlive: true},
				},
				{
					Path:      "settings",
					Component: "system/settings/index",
					Name:      "SystemSettings",
					Meta:      Meta{Title: "系统设置", Icon: "system", Roles: []string{"ADMIN"}, KeepAlive: true},
				},
				{
					Path:      "user/set",
					Component: "system/user/set",
					Name:      "Userset",
					Meta:      Meta{Title: "userset", Icon: "role", Hidden: true, Roles: []string{"ADMIN", "USER"}},
				},
			},
		},
	}
	c.JSON(200, gin.H{
		"code": "00000",
		"data": menus,
		"msg":  "获取成功",
	})
}
