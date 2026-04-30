package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
)

func init() {
	Register("discord", &DiscordProvider{})
}

// DiscordProvider implements OAuth for Discord
type DiscordProvider struct{}

type discordOAuthResponse struct {
	AccessToken  string `json:"access_token"`
	IDToken      string `json:"id_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
}

type discordUser struct {
	UID  string `json:"id"`
	ID   string `json:"username"`
	Name string `json:"global_name"`
}

type discordGuildMember struct {
	User  discordUser `json:"user"`
	Roles []string    `json:"roles"`
}

func (p *DiscordProvider) GetName() string {
	return "Discord"
}

func (p *DiscordProvider) IsEnabled() bool {
	return system_setting.GetDiscordSettings().Enabled
}

func (p *DiscordProvider) ExchangeToken(ctx context.Context, code string, c *gin.Context) (*OAuthToken, error) {
	if code == "" {
		return nil, NewOAuthError(i18n.MsgOAuthInvalidCode, nil)
	}

	logger.LogDebug(ctx, "[OAuth-Discord] ExchangeToken: code=%s...", code[:min(len(code), 10)])

	settings := system_setting.GetDiscordSettings()
	redirectUri := fmt.Sprintf("%s/oauth/discord", system_setting.ServerAddress)
	values := url.Values{}
	values.Set("client_id", settings.ClientId)
	values.Set("client_secret", settings.ClientSecret)
	values.Set("code", code)
	values.Set("grant_type", "authorization_code")
	values.Set("redirect_uri", redirectUri)

	logger.LogDebug(ctx, "[OAuth-Discord] ExchangeToken: redirect_uri=%s", redirectUri)

	req, err := http.NewRequestWithContext(ctx, "POST", "https://discord.com/api/v10/oauth2/token", strings.NewReader(values.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	client := http.Client{
		Timeout: 5 * time.Second,
	}
	res, err := client.Do(req)
	if err != nil {
		logger.LogError(ctx, fmt.Sprintf("[OAuth-Discord] ExchangeToken error: %s", err.Error()))
		return nil, NewOAuthErrorWithRaw(i18n.MsgOAuthConnectFailed, map[string]any{"Provider": "Discord"}, err.Error())
	}
	defer res.Body.Close()

	logger.LogDebug(ctx, "[OAuth-Discord] ExchangeToken response status: %d", res.StatusCode)

	var discordResponse discordOAuthResponse
	err = json.NewDecoder(res.Body).Decode(&discordResponse)
	if err != nil {
		logger.LogError(ctx, fmt.Sprintf("[OAuth-Discord] ExchangeToken decode error: %s", err.Error()))
		return nil, err
	}

	if discordResponse.AccessToken == "" {
		logger.LogError(ctx, "[OAuth-Discord] ExchangeToken failed: empty access token")
		return nil, NewOAuthError(i18n.MsgOAuthTokenFailed, map[string]any{"Provider": "Discord"})
	}

	logger.LogDebug(ctx, "[OAuth-Discord] ExchangeToken success: scope=%s", discordResponse.Scope)

	return &OAuthToken{
		AccessToken:  discordResponse.AccessToken,
		TokenType:    discordResponse.TokenType,
		RefreshToken: discordResponse.RefreshToken,
		ExpiresIn:    discordResponse.ExpiresIn,
		Scope:        discordResponse.Scope,
		IDToken:      discordResponse.IDToken,
	}, nil
}

func (p *DiscordProvider) GetUserInfo(ctx context.Context, token *OAuthToken) (*OAuthUser, error) {
	logger.LogDebug(ctx, "[OAuth-Discord] GetUserInfo: fetching user info")

	req, err := http.NewRequestWithContext(ctx, "GET", "https://discord.com/api/v10/users/@me", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)

	client := http.Client{
		Timeout: 5 * time.Second,
	}
	res, err := client.Do(req)
	if err != nil {
		logger.LogError(ctx, fmt.Sprintf("[OAuth-Discord] GetUserInfo error: %s", err.Error()))
		return nil, NewOAuthErrorWithRaw(i18n.MsgOAuthConnectFailed, map[string]any{"Provider": "Discord"}, err.Error())
	}
	defer res.Body.Close()

	logger.LogDebug(ctx, "[OAuth-Discord] GetUserInfo response status: %d", res.StatusCode)

	if res.StatusCode != http.StatusOK {
		logger.LogError(ctx, fmt.Sprintf("[OAuth-Discord] GetUserInfo failed: status=%d", res.StatusCode))
		return nil, NewOAuthError(i18n.MsgOAuthGetUserErr, nil)
	}

	var discordUser discordUser
	err = json.NewDecoder(res.Body).Decode(&discordUser)
	if err != nil {
		logger.LogError(ctx, fmt.Sprintf("[OAuth-Discord] GetUserInfo decode error: %s", err.Error()))
		return nil, err
	}

	if discordUser.UID == "" || discordUser.ID == "" {
		logger.LogError(ctx, "[OAuth-Discord] GetUserInfo failed: empty user fields")
		return nil, NewOAuthError(i18n.MsgOAuthUserInfoEmpty, map[string]any{"Provider": "Discord"})
	}

	logger.LogDebug(ctx, "[OAuth-Discord] GetUserInfo success: uid=%s, username=%s, name=%s", discordUser.UID, discordUser.ID, discordUser.Name)

	if err := validateDiscordGuildMember(ctx, token, discordUser.UID); err != nil {
		return nil, err
	}

	return &OAuthUser{
		ProviderUserID: discordUser.UID,
		Username:       discordUser.ID,
		DisplayName:    discordUser.Name,
	}, nil
}

func (p *DiscordProvider) IsUserIDTaken(providerUserID string) bool {
	return model.IsDiscordIdAlreadyTaken(providerUserID)
}

func (p *DiscordProvider) FillUserByProviderID(user *model.User, providerUserID string) error {
	user.DiscordId = providerUserID
	return user.FillUserByDiscordId()
}

func (p *DiscordProvider) SetProviderUserID(user *model.User, providerUserID string) {
	user.DiscordId = providerUserID
}

func (p *DiscordProvider) GetProviderPrefix() string {
	return "discord_"
}

func validateDiscordGuildMember(ctx context.Context, token *OAuthToken, userID string) error {
	if model.IsBlockedDiscordUser(userID) {
		logger.LogDebug(ctx, "[OAuth-Discord] user %s matched blocked user blacklist", userID)
		return &AccessDeniedError{Message: "Discord 登录失败：该用户已被禁止访问。"}
	}
	if isTrustedDiscordUser(userID) {
		logger.LogDebug(ctx, "[OAuth-Discord] user %s matched trusted user whitelist, skip guild and role checks", userID)
		return nil
	}

	guildID := strings.TrimSpace(getDiscordOption("discord.required_guild_id"))
	if guildID == "" {
		logger.LogError(ctx, "[OAuth-Discord] required guild id is not configured")
		return &AccessDeniedError{Message: "Discord 服务器校验未配置，请联系管理员。"}
	}

	member, err := fetchDiscordGuildMember(ctx, token.AccessToken, guildID)
	if err != nil {
		return err
	}
	if member.User.UID != "" && member.User.UID != userID {
		logger.LogError(ctx, fmt.Sprintf("[OAuth-Discord] guild member user mismatch: expected=%s actual=%s", userID, member.User.UID))
		return &AccessDeniedError{Message: "Discord 身份校验失败，请确认登录的是本人账号。"}
	}

	requiredRoleIDs := parseDiscordRoleIDs(getDiscordOption("discord.required_role_ids"))
	if len(requiredRoleIDs) == 0 {
		return nil
	}
	if hasAnyDiscordRole(member.Roles, requiredRoleIDs) {
		return nil
	}

	logger.LogDebug(ctx, "[OAuth-Discord] user %s is in guild %s but lacks required roles", userID, guildID)
	return &AccessDeniedError{Message: "Discord 身份组校验失败，请确认你拥有允许登录的服务器身份组。"}
}

func isTrustedDiscordUser(userID string) bool {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return false
	}
	trustedUserIDs := parseDiscordRoleIDs(getDiscordOption("discord.trusted_user_ids"))
	return trustedUserIDs[userID]
}

func fetchDiscordGuildMember(ctx context.Context, accessToken string, guildID string) (*discordGuildMember, error) {
	url := fmt.Sprintf("https://discord.com/api/v10/users/@me/guilds/%s/member", guildID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	client := http.Client{Timeout: 5 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		logger.LogError(ctx, fmt.Sprintf("[OAuth-Discord] guild member check error: %s", err.Error()))
		return nil, NewOAuthErrorWithRaw(i18n.MsgOAuthConnectFailed, map[string]any{"Provider": "Discord"}, err.Error())
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusForbidden || res.StatusCode == http.StatusNotFound {
		logger.LogDebug(ctx, "[OAuth-Discord] guild member check denied: status=%d", res.StatusCode)
		return nil, &AccessDeniedError{Message: "Discord 登录失败：你不属于指定服务器。"}
	}
	if res.StatusCode != http.StatusOK {
		logger.LogError(ctx, fmt.Sprintf("[OAuth-Discord] guild member check failed: status=%d", res.StatusCode))
		return nil, &AccessDeniedError{Message: "Discord 服务器成员校验失败，请稍后重试。"}
	}

	var member discordGuildMember
	if err := json.NewDecoder(res.Body).Decode(&member); err != nil {
		return nil, err
	}
	return &member, nil
}

func parseDiscordRoleIDs(value string) map[string]bool {
	roles := map[string]bool{}
	for _, roleID := range strings.Split(value, ",") {
		roleID = strings.TrimSpace(roleID)
		if roleID != "" {
			roles[roleID] = true
		}
	}
	return roles
}

func hasAnyDiscordRole(userRoles []string, requiredRoles map[string]bool) bool {
	for _, roleID := range userRoles {
		if requiredRoles[roleID] {
			return true
		}
	}
	return false
}

func getDiscordOption(key string) string {
	common.OptionMapRWMutex.RLock()
	value := common.OptionMap[key]
	common.OptionMapRWMutex.RUnlock()
	if value != "" {
		return value
	}
	switch key {
	case "discord.required_guild_id":
		return os.Getenv("DISCORD_REQUIRED_GUILD_ID")
	case "discord.required_role_ids":
		return os.Getenv("DISCORD_REQUIRED_ROLE_IDS")
	case "discord.trusted_user_ids":
		return os.Getenv("DISCORD_TRUSTED_USER_IDS")
	default:
		return ""
	}
}
