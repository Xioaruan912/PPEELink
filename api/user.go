package api

import (
	"strconv"
	"strings"

	"ppeelink/models"

	"github.com/gin-gonic/gin"
)

func currentUser(c *gin.Context) (*models.User, bool) {
	username, _ := c.Get("username")
	name, _ := username.(string)
	user := &models.User{Username: name}
	if name == "" || user.Find() != nil {
		return nil, false
	}
	return user, true
}

func roleCode(user *models.User) string {
	if user.IsAdmin() {
		return "ADMIN"
	}
	return "USER"
}

// validatePassword 新密码至少 8 位，且不能是默认密码或与用户名相同。
func validatePassword(password, username string) string {
	switch {
	case len(password) < 8:
		return "密码至少 8 位"
	case password == models.DefaultPassword:
		return "不能使用默认密码"
	case strings.EqualFold(password, username):
		return "密码不能与用户名相同"
	}
	return ""
}

// UserMe 当前账户信息；mustChangePassword 为真时前端只允许进入修改密码页。
func UserMe(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		c.JSON(401, gin.H{"code": 401, "msg": "账户不存在"})
		return
	}
	c.JSON(200, gin.H{
		"code": "00000",
		"data": gin.H{
			"nickname":           user.Nickname,
			"userId":             user.ID,
			"username":           user.Username,
			"roles":              []string{roleCode(user)},
			"mustChangePassword": user.MustChangePassword,
		},
		"msg": "获取用户信息成功",
	})
}

// UserSet 修改自己的用户名与密码。
func UserSet(c *gin.Context) {
	newUsername := strings.TrimSpace(c.PostForm("username"))
	newPassword := c.PostForm("password")
	if newUsername == "" || newPassword == "" {
		c.JSON(400, gin.H{"code": "00001", "msg": "用户名或密码不能为空"})
		return
	}
	if msg := validatePassword(newPassword, newUsername); msg != "" {
		c.JSON(400, gin.H{"code": "00002", "msg": msg})
		return
	}
	user, ok := currentUser(c)
	if !ok {
		c.JSON(401, gin.H{"code": 401, "msg": "账户不存在"})
		return
	}
	if newUsername != user.Username {
		var count int64
		models.DB.Model(&models.User{}).Where("username = ?", newUsername).Count(&count)
		if count > 0 {
			c.JSON(400, gin.H{"code": "00003", "msg": "用户名已被占用"})
			return
		}
	}
	if err := user.Set(&models.User{Username: newUsername, Password: newPassword}); err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "修改失败"})
		return
	}
	c.JSON(200, gin.H{"code": "00000", "msg": "修改成功"})
}

// ---- 管理员：用户管理 ------------------------------------------------------

type userView struct {
	ID                 int    `json:"id"`
	Username           string `json:"username"`
	Nickname           string `json:"nickname"`
	Role               string `json:"role"`
	Remark             string `json:"remark"`
	Disabled           bool   `json:"disabled"`
	MustChangePassword bool   `json:"mustChangePassword"`
	SubscriptionIDs    []int  `json:"subscriptionIds"`
}

func toUserView(u models.User) userView {
	ids := make([]int, 0, len(u.Subscriptions))
	for _, s := range u.Subscriptions {
		ids = append(ids, s.ID)
	}
	role := models.RoleUser
	if u.IsAdmin() {
		role = models.RoleAdmin
	}
	return userView{u.ID, u.Username, u.Nickname, role, u.Remark, u.Disabled, u.MustChangePassword, ids}
}

// UserPages 用户列表（含分配的订阅）。
func UserPages(c *gin.Context) {
	var users []models.User
	if err := models.DB.Preload("Subscriptions").Order("id asc").Find(&users).Error; err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "读取用户失败"})
		return
	}
	list := make([]userView, 0, len(users))
	for _, u := range users {
		list = append(list, toUserView(u))
	}
	c.JSON(200, gin.H{"code": "00000", "data": gin.H{"list": list}, "msg": "用户列表"})
}

type userSaveRequest struct {
	ID              int    `json:"id"`
	Username        string `json:"username"`
	Password        string `json:"password"`
	Nickname        string `json:"nickname"`
	Role            string `json:"role"`
	Remark          string `json:"remark"`
	Disabled        bool   `json:"disabled"`
	SubscriptionIDs []int  `json:"subscriptionIds"`
}

func assignSubscriptions(user *models.User, ids []int) error {
	subs := []models.Subcription{}
	if len(ids) > 0 {
		if err := models.DB.Where("id IN ?", ids).Find(&subs).Error; err != nil {
			return err
		}
	}
	return models.DB.Model(user).Association("Subscriptions").Replace(subs)
}

func normalizeRole(role string) string {
	if strings.EqualFold(role, models.RoleAdmin) {
		return models.RoleAdmin
	}
	return models.RoleUser
}

// countActiveAdmins 用于防止停用、降级或删除最后一个管理员。
func countActiveAdmins(excludeID int) int64 {
	var count int64
	models.DB.Model(&models.User{}).
		Where("(role = ? OR role = '' OR role IS NULL) AND disabled = ? AND id <> ?", models.RoleAdmin, false, excludeID).
		Count(&count)
	return count
}

// UserCreate 新建账户。新账户首次登录需修改密码。
func UserCreate(c *gin.Context) {
	var req userSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": "40000", "msg": "参数错误"})
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" || len(req.Password) < 8 {
		c.JSON(400, gin.H{"code": "40000", "msg": "用户名不能为空，初始密码至少 8 位"})
		return
	}
	var count int64
	models.DB.Model(&models.User{}).Where("username = ?", req.Username).Count(&count)
	if count > 0 {
		c.JSON(400, gin.H{"code": "40000", "msg": "用户名已存在"})
		return
	}
	user := &models.User{
		Username:           req.Username,
		Password:           req.Password,
		Nickname:           req.Nickname,
		Role:               normalizeRole(req.Role),
		Remark:             req.Remark,
		MustChangePassword: true,
	}
	if err := user.Create(); err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "创建失败"})
		return
	}
	if err := assignSubscriptions(user, req.SubscriptionIDs); err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "分配订阅失败"})
		return
	}
	c.JSON(200, gin.H{"code": "00000", "msg": "已创建"})
}

// UserEdit 修改资料、角色、状态与订阅分配；填写密码即重置密码。
func UserEdit(c *gin.Context) {
	var req userSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.ID == 0 {
		c.JSON(400, gin.H{"code": "40000", "msg": "参数错误"})
		return
	}
	var user models.User
	if err := models.DB.First(&user, req.ID).Error; err != nil {
		c.JSON(404, gin.H{"code": "40400", "msg": "用户不存在"})
		return
	}
	role := normalizeRole(req.Role)
	if user.IsAdmin() && (role != models.RoleAdmin || req.Disabled) && countActiveAdmins(user.ID) == 0 {
		c.JSON(400, gin.H{"code": "40000", "msg": "至少需要保留一个可用的管理员"})
		return
	}
	if me, ok := c.Get("userId"); ok && me == user.ID && req.Disabled {
		c.JSON(400, gin.H{"code": "40000", "msg": "不能停用自己"})
		return
	}
	updates := map[string]any{
		"nickname": req.Nickname,
		"role":     role,
		"remark":   req.Remark,
		"disabled": req.Disabled,
	}
	if req.Password != "" {
		if len(req.Password) < 8 {
			c.JSON(400, gin.H{"code": "40000", "msg": "重置密码至少 8 位"})
			return
		}
		hashed, err := models.HashPassword(req.Password)
		if err != nil {
			c.JSON(500, gin.H{"code": "50000", "msg": "重置密码失败"})
			return
		}
		updates["password"] = hashed
		updates["must_change_password"] = true
	}
	if err := models.DB.Model(&user).Updates(updates).Error; err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "保存失败"})
		return
	}
	if err := assignSubscriptions(&user, req.SubscriptionIDs); err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "分配订阅失败"})
		return
	}
	c.JSON(200, gin.H{"code": "00000", "msg": "已保存"})
}

// UserDelete 删除账户（不能删除自己或最后一个管理员）。
func UserDelete(c *gin.Context) {
	id, _ := strconv.Atoi(c.Query("id"))
	var user models.User
	if err := models.DB.First(&user, id).Error; err != nil {
		c.JSON(404, gin.H{"code": "40400", "msg": "用户不存在"})
		return
	}
	if me, ok := c.Get("userId"); ok && me == user.ID {
		c.JSON(400, gin.H{"code": "40000", "msg": "不能删除自己"})
		return
	}
	if user.IsAdmin() && countActiveAdmins(user.ID) == 0 {
		c.JSON(400, gin.H{"code": "40000", "msg": "至少需要保留一个可用的管理员"})
		return
	}
	_ = models.DB.Model(&user).Association("Subscriptions").Clear()
	if err := models.DB.Delete(&user).Error; err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "删除失败"})
		return
	}
	// 自建节点凭据随账户一起作废
	models.DB.Where("user_id = ?", user.ID).Delete(&models.ProxyAccount{})
	NotifyProxyChanged()
	c.JSON(200, gin.H{"code": "00000", "msg": "已删除"})
}

// ---- 我的订阅（所有登录账户） -------------------------------------------------

// MySubscriptions 普通用户看到分配给自己的订阅；管理员看到全部。
func MySubscriptions(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		c.JSON(401, gin.H{"code": 401, "msg": "账户不存在"})
		return
	}
	var subs []models.Subcription
	var err error
	if user.IsAdmin() {
		err = models.DB.Order("id asc").Find(&subs).Error
	} else {
		err = models.DB.Model(user).Association("Subscriptions").Find(&subs)
	}
	if err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "读取订阅失败"})
		return
	}
	base := baseURL(c)
	items := make([]gin.H, 0, len(subs))
	for i := range subs {
		_ = subs[i].EnsureToken()
		links, imports := subscriptionLinks(base, subs[i].Token)
		summary := trafficSummary{}
		summary.addExpire(subs[i].ExpiresAt)
		items = append(items, gin.H{
			"id":        subs[i].ID,
			"name":      subs[i].Name,
			"expiresAt": subs[i].ExpiresAt,
			"links":     links,
			"imports":   imports,
			"traffic": gin.H{
				"reported": summary.Reported,
				"upload":   summary.Upload,
				"download": summary.Download,
				"total":    summary.Total,
				"expire":   summary.Expire,
			},
		})
	}
	c.JSON(200, gin.H{"code": "00000", "data": items, "msg": "我的订阅"})
}
