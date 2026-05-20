package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

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

type discordGuild struct {
	ID string `json:"id"`
}

type discordGuildMember struct {
	Roles []string `json:"roles"`
}

func parseDiscordIdList(value string) []string {
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r' || r == '\t' || r == ' ' || r == ';'
	})
	ids := make([]string, 0, len(parts))
	seen := make(map[string]bool)
	for _, part := range parts {
		id := strings.TrimSpace(part)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	return ids
}

func discordIdInList(id string, ids []string) bool {
	for _, item := range ids {
		if id == item {
			return true
		}
	}
	return false
}

func discordHasAnyRole(userRoles []string, allowedRoles []string) bool {
	roleSet := make(map[string]bool, len(userRoles))
	for _, role := range userRoles {
		roleSet[role] = true
	}
	for _, role := range allowedRoles {
		if roleSet[role] {
			return true
		}
	}
	return false
}

func fetchDiscordGuildMembership(ctx context.Context, client *http.Client, accessToken string, guildID string) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://discord.com/api/v10/users/@me/guilds", nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	res, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return false, fmt.Errorf("discord guild membership check failed: status=%d", res.StatusCode)
	}

	var guilds []discordGuild
	if err := json.NewDecoder(res.Body).Decode(&guilds); err != nil {
		return false, err
	}

	for _, guild := range guilds {
		if guild.ID == guildID {
			return true, nil
		}
	}

	return false, nil
}

func fetchDiscordGuildMemberRoles(ctx context.Context, client *http.Client, accessToken string, guildID string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("https://discord.com/api/v10/users/@me/guilds/%s/member", url.PathEscape(guildID)), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("discord guild member role check failed: status=%d", res.StatusCode)
	}

	var member discordGuildMember
	if err := json.NewDecoder(res.Body).Decode(&member); err != nil {
		return nil, err
	}

	return member.Roles, nil
}

func validateDiscordAccess(ctx context.Context, client *http.Client, token *OAuthToken, userID string) error {
	settings := system_setting.GetDiscordSettings()

	if discordIdInList(userID, parseDiscordIdList(settings.WhitelistUserIds)) {
		logger.LogInfo(ctx, fmt.Sprintf("[OAuth-Discord] access granted by whitelist: uid=%s", userID))
		return nil
	}

	requiredGuildID := strings.TrimSpace(settings.RequiredGuildId)
	requiredRoleIDs := parseDiscordIdList(settings.RequiredRoleIds)

	if requiredGuildID == "" {
		if len(requiredRoleIDs) > 0 {
			return &AccessDeniedError{Message: "Discord 登录限制配置错误：设置身份组验证时必须同时设置服务器 ID"}
		}
		return nil
	}

	inGuild, err := fetchDiscordGuildMembership(ctx, client, token.AccessToken, requiredGuildID)
	if err != nil {
		logger.LogError(ctx, fmt.Sprintf("[OAuth-Discord] guild membership check error: %s", err.Error()))
		return &AccessDeniedError{Message: "Discord 服务器成员验证失败，请确认授权包含 guilds 权限后重试"}
	}
	if !inGuild {
		return &AccessDeniedError{Message: "你的 Discord 账号不在指定服务器中，无法登录"}
	}

	if len(requiredRoleIDs) == 0 {
		return nil
	}

	userRoles, err := fetchDiscordGuildMemberRoles(ctx, client, token.AccessToken, requiredGuildID)
	if err != nil {
		logger.LogError(ctx, fmt.Sprintf("[OAuth-Discord] guild member role check error: %s", err.Error()))
		return &AccessDeniedError{Message: "Discord 身份组验证失败，请确认授权包含 guilds.members.read 权限后重试"}
	}
	if !discordHasAnyRole(userRoles, requiredRoleIDs) {
		return &AccessDeniedError{Message: "你的 Discord 账号缺少允许登录的服务器身份组"}
	}

	return nil
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

	if err := validateDiscordAccess(ctx, &client, token, discordUser.UID); err != nil {
		return nil, err
	}

	logger.LogDebug(ctx, "[OAuth-Discord] GetUserInfo success: uid=%s, username=%s, name=%s", discordUser.UID, discordUser.ID, discordUser.Name)

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
