package api

import (
	"errors"
	"github.com/gin-gonic/gin"
	"ppeelink/models"
	"time"
)

// Even IDs identify users; odd IDs identify API tokens. These namespaces never overlap.
func terminalOwner(c *gin.Context) (int, error) {
	owner := 0
	switch c.GetString("authType") {
	case "jwt":
		owner = 2 * c.GetInt("userId")
	case "api-token":
		owner = 2*int(c.GetUint("apiTokenId")) + 1
	}
	if !terminalOwnerValid(owner) {
		return 0, errors.New("终端需要有效的管理员身份")
	}
	return owner, nil
}
func terminalOwnerValid(owner int) bool {
	if owner <= 1 {
		return false
	}
	if owner%2 == 0 {
		var user models.User
		return models.DB.First(&user, owner/2).Error == nil && user.IsAdmin() && !user.Disabled && !user.MustChangePassword
	}
	var token models.APIToken
	return models.DB.First(&token, (owner-1)/2).Error == nil && token.Enabled && token.HasScope("admin") && (token.ExpiresAt == nil || token.ExpiresAt.After(time.Now()))
}
func takeTerminalTicket(raw string) (monitorSSHTicket, bool) {
	value, ok := monitorSSHTickets.LoadAndDelete(raw)
	if !ok {
		return monitorSSHTicket{}, false
	}
	ticket, ok := value.(monitorSSHTicket)
	if !ok || time.Now().After(ticket.ExpiresAt) || !terminalOwnerValid(ticket.Owner) {
		return monitorSSHTicket{}, false
	}
	return ticket, true
}
