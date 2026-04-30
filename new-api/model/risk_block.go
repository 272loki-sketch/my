package model

import (
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

const (
	RiskBlockedDiscordUserIDsKey = "risk.blocked_discord_user_ids"
	RiskBlockedIPsKey            = "risk.blocked_ips"
)

func IsBlockedDiscordUser(discordUserID string) bool {
	discordUserID = strings.TrimSpace(discordUserID)
	if discordUserID == "" {
		return false
	}
	return parseRiskList(getRiskOption(RiskBlockedDiscordUserIDsKey, "RISK_BLOCKED_DISCORD_USER_IDS"))[discordUserID]
}

func IsBlockedIP(clientIP string) bool {
	clientIP = strings.TrimSpace(clientIP)
	if clientIP == "" {
		return false
	}
	ip := net.ParseIP(clientIP)
	if ip == nil {
		return false
	}
	for raw := range parseRiskList(getRiskOption(RiskBlockedIPsKey, "RISK_BLOCKED_IPS")) {
		if strings.Contains(raw, "/") {
			_, network, err := net.ParseCIDR(raw)
			if err == nil && network.Contains(ip) {
				return true
			}
			continue
		}
		blockedIP := net.ParseIP(raw)
		if blockedIP != nil && blockedIP.Equal(ip) {
			return true
		}
	}
	return false
}

func DisableUserForRisk(userId int, reason string) error {
	if userId <= 0 {
		return nil
	}
	var user User
	if err := DB.Select("id", "role", "status").Where("id = ?", userId).First(&user).Error; err != nil {
		return err
	}
	if user.Role >= common.RoleRootUser {
		common.SysLog(fmt.Sprintf("skip disabling root user %d for risk block: %s", userId, reason))
		return nil
	}
	if user.Status == common.UserStatusDisabled {
		return nil
	}
	if err := DB.Model(&User{}).Where("id = ?", userId).Update("status", common.UserStatusDisabled).Error; err != nil {
		return err
	}
	_ = invalidateUserCache(userId)
	RecordLog(userId, LogTypeSystem, "风控黑名单自动封禁："+reason)
	return nil
}

func getRiskOption(key string, envKey string) string {
	common.OptionMapRWMutex.RLock()
	value := common.OptionMap[key]
	common.OptionMapRWMutex.RUnlock()
	if strings.TrimSpace(value) == "" {
		value = os.Getenv(envKey)
	}
	return value
}

func parseRiskList(value string) map[string]bool {
	items := map[string]bool{}
	for _, raw := range strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r' || r == '\t' || r == ' '
	}) {
		raw = strings.TrimSpace(raw)
		if raw != "" {
			items[raw] = true
		}
	}
	return items
}
