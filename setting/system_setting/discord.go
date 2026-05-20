package system_setting

import "github.com/QuantumNous/new-api/setting/config"

type DiscordSettings struct {
	Enabled          bool   `json:"enabled"`
	ClientId         string `json:"client_id"`
	ClientSecret     string `json:"client_secret"`
	RequiredGuildId  string `json:"required_guild_id"`
	RequiredRoleIds  string `json:"required_role_ids"`
	WhitelistUserIds string `json:"whitelist_user_ids"`
}

// 默认配置
var defaultDiscordSettings = DiscordSettings{}

func init() {
	// 注册到全局配置管理器
	config.GlobalConfig.Register("discord", &defaultDiscordSettings)
}

func GetDiscordSettings() *DiscordSettings {
	return &defaultDiscordSettings
}
