package models

import (
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// 角色：admin 管理全部；user 只能查看分配给自己的订阅。
const (
	RoleAdmin = "admin"
	RoleUser  = "user"
	// DefaultPassword 初始化管理员的密码；仍在使用它的账户登录后必须先改密码。
	DefaultPassword = "123456"
)

type User struct {
	ID                 int
	Username           string
	Password           string `json:"-"`
	Role               string
	Nickname           string
	Remark             string
	Disabled           bool          `gorm:"not null;default:false"`
	MustChangePassword bool          `gorm:"not null;default:false"`
	Subscriptions      []Subcription `gorm:"many2many:user_subscriptions;" json:"-"`
}

// IsAdmin 旧数据的角色可能为空，按管理员处理以免升级后被锁在门外。
func (user *User) IsAdmin() bool {
	return user.Role == "" || strings.EqualFold(user.Role, RoleAdmin)
}

func passwordIsHashed(value string) bool {
	return strings.HasPrefix(value, "$2a$") || strings.HasPrefix(value, "$2b$") || strings.HasPrefix(value, "$2y$")
}

func HashPassword(value string) (string, error) {
	if value == "" {
		return "", fmt.Errorf("password empty")
	}
	if passwordIsHashed(value) {
		return value, nil
	}
	out, err := bcrypt.GenerateFromPassword([]byte(value), bcrypt.DefaultCost)
	return string(out), err
}

func (user *User) Create() error { // 创建用户
	hashed, err := HashPassword(user.Password)
	if err != nil {
		return err
	}
	user.Password = hashed
	return DB.Create(user).Error
}
func (user *User) Set(UpdateUser *User) error { // 设置用户
	updates := map[string]any{}
	if UpdateUser.Username != "" {
		updates["username"] = UpdateUser.Username
	}
	if UpdateUser.Password != "" {
		hashed, err := HashPassword(UpdateUser.Password)
		if err != nil {
			return err
		}
		updates["password"] = hashed
		// 改过密码即解除强制修改
		updates["must_change_password"] = false
	}
	if len(updates) == 0 {
		return nil
	}
	return DB.Model(&User{}).Where("username = ?", user.Username).Updates(updates).Error
}
func (user *User) Verify() error { // 验证用户
	plain := user.Password
	var stored User
	if err := DB.Where("username = ?", user.Username).First(&stored).Error; err != nil {
		return err
	}
	if passwordIsHashed(stored.Password) {
		if err := bcrypt.CompareHashAndPassword([]byte(stored.Password), []byte(plain)); err != nil {
			return err
		}
	} else {
		if stored.Password != plain {
			return fmt.Errorf("password mismatch")
		}
		hashed, err := HashPassword(plain)
		if err == nil {
			_ = DB.Model(&stored).Update("password", hashed).Error
			stored.Password = hashed
		}
	}
	if stored.Disabled {
		return fmt.Errorf("user disabled")
	}
	if plain == DefaultPassword && !stored.MustChangePassword {
		_ = DB.Model(&stored).Update("must_change_password", true).Error
		stored.MustChangePassword = true
	}
	*user = stored
	return nil
}

func (user *User) Find() error { // 查找用户
	return DB.Where("username = ? ", user.Username).First(user).Error
}

func (user *User) All() ([]User, error) { // 获取所有用户
	var users []User
	err := DB.Find(&users).Error
	return users, err
}

func (user *User) Del() error { // 删除用户
	return DB.Delete(user).Error
}
