package model

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"

	"github.com/bytedance/gopkg/util/gopool"
	"gorm.io/gorm"
)

type Log struct {
	Id               int    `json:"id" gorm:"index:idx_created_at_id,priority:1;index:idx_user_id_id,priority:2"`
	UserId           int    `json:"user_id" gorm:"index;index:idx_user_id_id,priority:1"`
	CreatedAt        int64  `json:"created_at" gorm:"bigint;index:idx_created_at_id,priority:2;index:idx_created_at_type"`
	Type             int    `json:"type" gorm:"index:idx_created_at_type"`
	Content          string `json:"content"`
	Username         string `json:"username" gorm:"index;index:index_username_model_name,priority:2;default:''"`
	TokenName        string `json:"token_name" gorm:"index;default:''"`
	ModelName        string `json:"model_name" gorm:"index;index:index_username_model_name,priority:1;default:''"`
	Quota            int    `json:"quota" gorm:"default:0"`
	PromptTokens     int    `json:"prompt_tokens" gorm:"default:0"`
	CompletionTokens int    `json:"completion_tokens" gorm:"default:0"`
	UseTime          int    `json:"use_time" gorm:"default:0"`
	IsStream         bool   `json:"is_stream"`
	ChannelId        int    `json:"channel" gorm:"index"`
	ChannelName      string `json:"channel_name" gorm:"->"`
	TokenId          int    `json:"token_id" gorm:"default:0;index"`
	Group            string `json:"group" gorm:"index"`
	Ip               string `json:"ip" gorm:"index;default:''"`
	RequestId        string `json:"request_id,omitempty" gorm:"type:varchar(64);index:idx_logs_request_id;default:''"`
	Other            string `json:"other"`
}

// don't use iota, avoid change log type value
const (
	LogTypeUnknown = 0
	LogTypeTopup   = 1
	LogTypeConsume = 2
	LogTypeManage  = 3
	LogTypeSystem  = 4
	LogTypeError   = 5
	LogTypeRefund  = 6
)

func formatUserLogs(logs []*Log, startIdx int) {
	for i := range logs {
		logs[i].ChannelName = ""
		var otherMap map[string]interface{}
		otherMap, _ = common.StrToMap(logs[i].Other)
		if otherMap != nil {
			// Remove admin-only debug fields.
			delete(otherMap, "admin_info")
			delete(otherMap, "request_headers")
			delete(otherMap, "client_software")
			// delete(otherMap, "reject_reason")
			delete(otherMap, "stream_status")
		}
		logs[i].Other = common.MapToJsonStr(otherMap)
		logs[i].Id = startIdx + i + 1
	}
}

func FormatLogsForRole(logs []*Log, role int) {
	if role >= common.RoleRootUser {
		return
	}
	for i := range logs {
		otherMap, _ := common.StrToMap(logs[i].Other)
		if otherMap == nil {
			continue
		}
		delete(otherMap, "request_headers")
		delete(otherMap, "client_software")
		logs[i].Other = common.MapToJsonStr(otherMap)
	}
}

func GetLogByTokenId(tokenId int) (logs []*Log, err error) {
	err = LOG_DB.Model(&Log{}).Where("token_id = ?", tokenId).Order("id desc").Limit(common.MaxRecentItems).Find(&logs).Error
	formatUserLogs(logs, 0)
	return logs, err
}

func RecordLog(userId int, logType int, content string) {
	if logType == LogTypeConsume && !common.LogConsumeEnabled {
		return
	}
	username, _ := GetUsernameById(userId, false)
	log := &Log{
		UserId:    userId,
		Username:  username,
		CreatedAt: common.GetTimestamp(),
		Type:      logType,
		Content:   content,
	}
	err := LOG_DB.Create(log).Error
	if err != nil {
		common.SysLog("failed to record log: " + err.Error())
	}
}

// RecordLogWithAdminInfo 记录操作日志，并将管理员相关信息存入 Other.admin_info，
func RecordLogWithAdminInfo(userId int, logType int, content string, adminInfo map[string]interface{}) {
	if logType == LogTypeConsume && !common.LogConsumeEnabled {
		return
	}
	username, _ := GetUsernameById(userId, false)
	log := &Log{
		UserId:    userId,
		Username:  username,
		CreatedAt: common.GetTimestamp(),
		Type:      logType,
		Content:   content,
	}
	if len(adminInfo) > 0 {
		other := map[string]interface{}{
			"admin_info": adminInfo,
		}
		log.Other = common.MapToJsonStr(other)
	}
	if err := LOG_DB.Create(log).Error; err != nil {
		common.SysLog("failed to record log: " + err.Error())
	}
}

func RecordTopupLog(userId int, content string, callerIp string, paymentMethod string, callbackPaymentMethod string) {
	username, _ := GetUsernameById(userId, false)
	adminInfo := map[string]interface{}{
		"server_ip":               common.GetIp(),
		"node_name":               common.NodeName,
		"caller_ip":               callerIp,
		"payment_method":          paymentMethod,
		"callback_payment_method": callbackPaymentMethod,
		"version":                 common.Version,
	}
	other := map[string]interface{}{
		"admin_info": adminInfo,
	}
	log := &Log{
		UserId:    userId,
		Username:  username,
		CreatedAt: common.GetTimestamp(),
		Type:      LogTypeTopup,
		Content:   content,
		Ip:        callerIp,
		Other:     common.MapToJsonStr(other),
	}
	err := LOG_DB.Create(log).Error
	if err != nil {
		common.SysLog("failed to record topup log: " + err.Error())
	}
}

func RecordErrorLog(c *gin.Context, userId int, channelId int, modelName string, tokenName string, content string, tokenId int, useTimeSeconds int,
	isStream bool, group string, other map[string]interface{}) {
	logger.LogInfo(c, fmt.Sprintf("record error log: userId=%d, channelId=%d, modelName=%s, tokenName=%s, content=%s", userId, channelId, modelName, tokenName, content))
	username := c.GetString("username")
	requestId := c.GetString(common.RequestIdKey)
	other = withRequestHeaderAudit(c, other)
	otherStr := common.MapToJsonStr(other)
	log := &Log{
		UserId:           userId,
		Username:         username,
		CreatedAt:        common.GetTimestamp(),
		Type:             LogTypeError,
		Content:          content,
		PromptTokens:     0,
		CompletionTokens: 0,
		TokenName:        tokenName,
		ModelName:        modelName,
		Quota:            0,
		ChannelId:        channelId,
		TokenId:          tokenId,
		UseTime:          useTimeSeconds,
		IsStream:         isStream,
		Group:            group,
		Ip:               c.ClientIP(),
		RequestId:        requestId,
		Other:            otherStr,
	}
	err := LOG_DB.Create(log).Error
	if err != nil {
		logger.LogError(c, "failed to record log: "+err.Error())
	}
}

type RecordConsumeLogParams struct {
	ChannelId        int                    `json:"channel_id"`
	PromptTokens     int                    `json:"prompt_tokens"`
	CompletionTokens int                    `json:"completion_tokens"`
	ModelName        string                 `json:"model_name"`
	TokenName        string                 `json:"token_name"`
	Quota            int                    `json:"quota"`
	Content          string                 `json:"content"`
	TokenId          int                    `json:"token_id"`
	UseTimeSeconds   int                    `json:"use_time_seconds"`
	IsStream         bool                   `json:"is_stream"`
	Group            string                 `json:"group"`
	Other            map[string]interface{} `json:"other"`
}

func RecordConsumeLog(c *gin.Context, userId int, params RecordConsumeLogParams) {
	if !common.LogConsumeEnabled {
		return
	}
	logger.LogInfo(c, fmt.Sprintf("record consume log: userId=%d, params=%s", userId, common.GetJsonString(params)))
	username := c.GetString("username")
	requestId := c.GetString(common.RequestIdKey)
	params.Other = withRequestHeaderAudit(c, params.Other)
	otherStr := common.MapToJsonStr(params.Other)
	log := &Log{
		UserId:           userId,
		Username:         username,
		CreatedAt:        common.GetTimestamp(),
		Type:             LogTypeConsume,
		Content:          params.Content,
		PromptTokens:     params.PromptTokens,
		CompletionTokens: params.CompletionTokens,
		TokenName:        params.TokenName,
		ModelName:        params.ModelName,
		Quota:            params.Quota,
		ChannelId:        params.ChannelId,
		TokenId:          params.TokenId,
		UseTime:          params.UseTimeSeconds,
		IsStream:         params.IsStream,
		Group:            params.Group,
		Ip:               c.ClientIP(),
		RequestId:        requestId,
		Other:            otherStr,
	}
	err := LOG_DB.Create(log).Error
	if err != nil {
		logger.LogError(c, "failed to record log: "+err.Error())
	}
	if common.DataExportEnabled {
		gopool.Go(func() {
			LogQuotaData(userId, username, params.ModelName, params.Quota, common.GetTimestamp(), params.PromptTokens+params.CompletionTokens)
		})
	}
}

func withRequestHeaderAudit(c *gin.Context, other map[string]interface{}) map[string]interface{} {
	if other == nil {
		other = map[string]interface{}{}
	}
	headers := sanitizedRequestHeaders(c)
	if len(headers) > 0 {
		other["request_headers"] = headers
	}
	if client := detectClientSoftware(headers); client != "" {
		other["client_software"] = client
	}
	return other
}

func sanitizedRequestHeaders(c *gin.Context) map[string]string {
	if c == nil || c.Request == nil {
		return nil
	}
	headers := map[string]string{}
	for key, values := range c.Request.Header {
		name := strings.TrimSpace(key)
		if name == "" || isSensitiveHeader(name) {
			continue
		}
		value := strings.TrimSpace(strings.Join(values, ", "))
		if value == "" {
			continue
		}
		if len(value) > 500 {
			value = value[:500] + "..."
		}
		headers[name] = value
	}
	return headers
}

func isSensitiveHeader(header string) bool {
	switch strings.ToLower(header) {
	case "authorization", "cookie", "set-cookie", "x-api-key", "api-key", "proxy-authorization", "x-goog-api-key", "cf-access-client-secret":
		return true
	default:
		return false
	}
}

func detectClientSoftware(headers map[string]string) string {
	if len(headers) == 0 {
		return ""
	}
	keys := []string{"User-Agent", "X-Title", "HTTP-Referer", "Referer", "Origin", "X-Client", "X-Client-Name", "X-SillyTavern-Version", "X-Stainless-Package-Version"}
	for _, key := range keys {
		for headerKey, value := range headers {
			if strings.EqualFold(headerKey, key) && strings.TrimSpace(value) != "" {
				return strings.TrimSpace(value)
			}
		}
	}
	return ""
}

type RecordTaskBillingLogParams struct {
	UserId    int
	LogType   int
	Content   string
	ChannelId int
	ModelName string
	Quota     int
	TokenId   int
	Group     string
	Other     map[string]interface{}
}

func RecordTaskBillingLog(params RecordTaskBillingLogParams) {
	if params.LogType == LogTypeConsume && !common.LogConsumeEnabled {
		return
	}
	username, _ := GetUsernameById(params.UserId, false)
	tokenName := ""
	if params.TokenId > 0 {
		if token, err := GetTokenById(params.TokenId); err == nil {
			tokenName = token.Name
		}
	}
	log := &Log{
		UserId:    params.UserId,
		Username:  username,
		CreatedAt: common.GetTimestamp(),
		Type:      params.LogType,
		Content:   params.Content,
		TokenName: tokenName,
		ModelName: params.ModelName,
		Quota:     params.Quota,
		ChannelId: params.ChannelId,
		TokenId:   params.TokenId,
		Group:     params.Group,
		Other:     common.MapToJsonStr(params.Other),
	}
	err := LOG_DB.Create(log).Error
	if err != nil {
		common.SysLog("failed to record task billing log: " + err.Error())
	}
}

func GetAllLogs(logType int, startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string, startIdx int, num int, channel int, group string, requestId string, ip string) (logs []*Log, total int64, err error) {
	var tx *gorm.DB
	if logType == LogTypeUnknown {
		tx = LOG_DB
	} else {
		tx = LOG_DB.Where("logs.type = ?", logType)
	}

	if modelName != "" {
		tx = tx.Where("logs.model_name like ?", modelName)
	}
	if username != "" {
		tx = tx.Where("logs.username = ?", username)
	}
	if tokenName != "" {
		tx = tx.Where("logs.token_name = ?", tokenName)
	}
	if requestId != "" {
		tx = tx.Where("logs.request_id = ?", requestId)
	}
	if startTimestamp != 0 {
		tx = tx.Where("logs.created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("logs.created_at <= ?", endTimestamp)
	}
	if channel != 0 {
		tx = tx.Where("logs.channel_id = ?", channel)
	}
	if group != "" {
		tx = tx.Where("logs."+logGroupCol+" = ?", group)
	}
	if ip != "" {
		tx = tx.Where("logs.ip = ?", ip)
	}
	err = tx.Model(&Log{}).Count(&total).Error
	if err != nil {
		return nil, 0, err
	}
	err = tx.Order("logs.id desc").Limit(num).Offset(startIdx).Find(&logs).Error
	if err != nil {
		return nil, 0, err
	}

	channelIds := types.NewSet[int]()
	for _, log := range logs {
		if log.ChannelId != 0 {
			channelIds.Add(log.ChannelId)
		}
	}

	if channelIds.Len() > 0 {
		var channels []struct {
			Id   int    `gorm:"column:id"`
			Name string `gorm:"column:name"`
		}
		if common.MemoryCacheEnabled {
			// Cache get channel
			for _, channelId := range channelIds.Items() {
				if cacheChannel, err := CacheGetChannel(channelId); err == nil {
					channels = append(channels, struct {
						Id   int    `gorm:"column:id"`
						Name string `gorm:"column:name"`
					}{
						Id:   channelId,
						Name: cacheChannel.Name,
					})
				}
			}
		} else {
			// Bulk query channels from DB
			if err = DB.Table("channels").Select("id, name").Where("id IN ?", channelIds.Items()).Find(&channels).Error; err != nil {
				return logs, total, err
			}
		}
		channelMap := make(map[int]string, len(channels))
		for _, channel := range channels {
			channelMap[channel.Id] = channel.Name
		}
		for i := range logs {
			logs[i].ChannelName = channelMap[logs[i].ChannelId]
		}
	}

	return logs, total, err
}

type SuspiciousUser struct {
	UserId             int      `json:"user_id"`
	Username           string   `json:"username"`
	Remark             string   `json:"remark"`
	RequestCount       int      `json:"request_count"`
	IpCount            int      `json:"ip_count"`
	SharedIpUserCount  int      `json:"shared_ip_user_count"`
	Ips                []string `json:"ips"`
	ActiveHourCount    int      `json:"active_hour_count"`
	MaxHourlyRequests  int      `json:"max_hourly_requests"`
	NearLimitHourCount int      `json:"near_limit_hour_count"`
	FirstSeen          int64    `json:"first_seen"`
	LastSeen           int64    `json:"last_seen"`
	SampleClient       string   `json:"sample_client"`
	Reasons            []string `json:"reasons"`
	Score              int      `json:"score"`
}

type suspiciousUserAgg struct {
	UserId    int
	Username  string
	Requests  int
	FirstSeen int64
	LastSeen  int64
	Ips       map[string]bool
	Hours     map[int64]int
	Clients   map[string]int
	SharedIPs map[string]int
}

type SuspiciousUserNote struct {
	UserId    int    `json:"user_id" gorm:"primaryKey;column:user_id"`
	Remark    string `json:"remark" gorm:"type:varchar(500);default:''"`
	UpdatedAt int64  `json:"updated_at" gorm:"bigint"`
}

func GetSuspiciousUsers(startTimestamp int64, endTimestamp int64, minScore int, limit int) ([]SuspiciousUser, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	if minScore <= 0 {
		minScore = 1
	}
	if endTimestamp == 0 {
		endTimestamp = common.GetTimestamp()
	}
	if startTimestamp == 0 {
		startTimestamp = endTimestamp - 7*24*60*60
	}

	var logs []*Log
	err := LOG_DB.Model(&Log{}).
		Where("type = ? AND created_at >= ? AND created_at <= ?", LogTypeConsume, startTimestamp, endTimestamp).
		Order("created_at asc").
		Limit(20000).
		Find(&logs).Error
	if err != nil {
		return nil, err
	}

	trustedUserIDs := getTrustedRiskUserIDs()
	aggs := map[int]*suspiciousUserAgg{}
	ipUsers := map[string]map[int]bool{}
	for _, log := range logs {
		if log.UserId == 0 {
			continue
		}
		if trustedUserIDs[log.UserId] {
			continue
		}
		agg := aggs[log.UserId]
		if agg == nil {
			agg = &suspiciousUserAgg{
				UserId:   log.UserId,
				Username: log.Username,
				Ips:      map[string]bool{},
				Hours:    map[int64]int{},
				Clients:  map[string]int{},
			}
			aggs[log.UserId] = agg
		}
		agg.Requests++
		if agg.Username == "" && log.Username != "" {
			agg.Username = log.Username
		}
		if agg.FirstSeen == 0 || log.CreatedAt < agg.FirstSeen {
			agg.FirstSeen = log.CreatedAt
		}
		if log.CreatedAt > agg.LastSeen {
			agg.LastSeen = log.CreatedAt
		}
		if log.Ip != "" {
			agg.Ips[log.Ip] = true
			if ipUsers[log.Ip] == nil {
				ipUsers[log.Ip] = map[int]bool{}
			}
			ipUsers[log.Ip][log.UserId] = true
		}
		agg.Hours[log.CreatedAt/3600]++
		if client := extractClientSoftware(log.Other); client != "" {
			agg.Clients[client]++
		}
	}
	for _, agg := range aggs {
		agg.SharedIPs = map[string]int{}
		for ip := range agg.Ips {
			userCount := len(ipUsers[ip])
			if userCount > 1 {
				agg.SharedIPs[ip] = userCount
			}
		}
	}

	result := make([]SuspiciousUser, 0)
	for _, agg := range aggs {
		su := buildSuspiciousUser(agg)
		if su.Score >= minScore {
			result = append(result, su)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Score == result[j].Score {
			return result[i].RequestCount > result[j].RequestCount
		}
		return result[i].Score > result[j].Score
	})
	if len(result) > limit {
		result = result[:limit]
	}
	fillSuspiciousUserRemarks(result)
	return result, nil
}

func fillSuspiciousUserRemarks(users []SuspiciousUser) {
	if len(users) == 0 {
		return
	}
	userIDs := make([]int, 0, len(users))
	for _, user := range users {
		userIDs = append(userIDs, user.UserId)
	}
	var notes []SuspiciousUserNote
	if err := DB.Where("user_id IN ?", userIDs).Find(&notes).Error; err != nil {
		common.SysError("failed to load suspicious user notes: " + err.Error())
		return
	}
	noteMap := make(map[int]string, len(notes))
	for _, note := range notes {
		noteMap[note.UserId] = note.Remark
	}
	for i := range users {
		users[i].Remark = noteMap[users[i].UserId]
	}
}

func UpdateSuspiciousUserRemark(userId int, remark string) error {
	if userId <= 0 {
		return errors.New("用户 ID 无效")
	}
	if len([]rune(remark)) > 500 {
		return errors.New("备注不能超过 500 个字符")
	}
	note := SuspiciousUserNote{
		UserId:    userId,
		Remark:    remark,
		UpdatedAt: common.GetTimestamp(),
	}
	return DB.Save(&note).Error
}

func getTrustedRiskUserIDs() map[int]bool {
	common.OptionMapRWMutex.RLock()
	value := common.OptionMap["risk.trusted_user_ids"]
	common.OptionMapRWMutex.RUnlock()
	if strings.TrimSpace(value) == "" {
		value = os.Getenv("TRUSTED_USER_IDS")
	}
	ids := map[int]bool{}
	for _, rawID := range strings.Split(value, ",") {
		rawID = strings.TrimSpace(rawID)
		if rawID == "" {
			continue
		}
		id, err := strconv.Atoi(rawID)
		if err == nil && id > 0 {
			ids[id] = true
		}
	}
	return ids
}

func buildSuspiciousUser(agg *suspiciousUserAgg) SuspiciousUser {
	ips := make([]string, 0, len(agg.Ips))
	for ip := range agg.Ips {
		ips = append(ips, ip)
	}
	sort.Strings(ips)

	activeHours := len(agg.Hours)
	maxHourlyRequests := 0
	nearLimitHourCount := 0
	nearLimitThreshold := getSuspiciousNearLimitThreshold()
	for _, count := range agg.Hours {
		if count > maxHourlyRequests {
			maxHourlyRequests = count
		}
		if nearLimitThreshold > 0 && count >= nearLimitThreshold {
			nearLimitHourCount++
		}
	}

	reasons := []string{}
	score := 0
	if len(ips) > 9 {
		score += 3
		reasons = append(reasons, fmt.Sprintf("单用户使用 IP 超过 9 个（%d 个）", len(ips)))
	}
	maxSharedIpUserCount := 0
	for _, userCount := range agg.SharedIPs {
		if userCount > maxSharedIpUserCount {
			maxSharedIpUserCount = userCount
		}
	}
	if maxSharedIpUserCount >= 5 {
		score += 3
		reasons = append(reasons, fmt.Sprintf("疑似云酒馆（付费云酒馆：同 IP 关联多个用户，最多 %d 个用户）", maxSharedIpUserCount))
	} else if maxSharedIpUserCount >= 2 {
		score += 1
		reasons = append(reasons, fmt.Sprintf("疑似云酒馆（付费云酒馆：同 IP 关联多个用户，最多 %d 个用户）", maxSharedIpUserCount))
	}
	if len(ips) == 1 && maxSharedIpUserCount <= 1 && (activeHours >= 6 || agg.Requests >= 120) {
		score += 2
		reasons = append(reasons, fmt.Sprintf("疑似云酒馆（自部署云酒馆：长期固定单 IP 使用，%d 个活跃小时，%d 次请求）", activeHours, agg.Requests))
	}
	if activeHours >= 20 {
		score += 4
		reasons = append(reasons, fmt.Sprintf("接近全天不间断使用（%d 个活跃小时）", activeHours))
	} else if activeHours >= 12 {
		score += 2
		reasons = append(reasons, fmt.Sprintf("长时间连续使用（%d 个活跃小时）", activeHours))
	}
	if nearLimitHourCount >= 6 {
		score += 3
		reasons = append(reasons, fmt.Sprintf("多次接近限速使用（%d 个小时达到阈值）", nearLimitHourCount))
	} else if nearLimitHourCount >= 3 {
		score += 1
		reasons = append(reasons, fmt.Sprintf("频繁接近限速使用（%d 个小时达到阈值）", nearLimitHourCount))
	}
	if agg.Requests >= 500 {
		score += 2
		reasons = append(reasons, fmt.Sprintf("请求量异常偏高（%d 次）", agg.Requests))
	} else if agg.Requests >= 200 {
		score += 1
		reasons = append(reasons, fmt.Sprintf("请求量较高（%d 次）", agg.Requests))
	}

	return SuspiciousUser{
		UserId:             agg.UserId,
		Username:           agg.Username,
		RequestCount:       agg.Requests,
		IpCount:            len(ips),
		SharedIpUserCount:  maxSharedIpUserCount,
		Ips:                ips,
		ActiveHourCount:    activeHours,
		MaxHourlyRequests:  maxHourlyRequests,
		NearLimitHourCount: nearLimitHourCount,
		FirstSeen:          agg.FirstSeen,
		LastSeen:           agg.LastSeen,
		SampleClient:       mostCommonString(agg.Clients),
		Reasons:            reasons,
		Score:              score,
	}
}

func getSuspiciousNearLimitThreshold() int {
	if setting.ModelRequestRateLimitEnabled && setting.ModelRequestRateLimitCount > 0 && setting.ModelRequestRateLimitDurationMinutes > 0 {
		perHour := setting.ModelRequestRateLimitCount * 60 / setting.ModelRequestRateLimitDurationMinutes
		threshold := perHour * 8 / 10
		if threshold > 0 {
			return threshold
		}
	}
	return 60
}

func extractClientSoftware(other string) string {
	otherMap, _ := common.StrToMap(other)
	if otherMap == nil {
		return ""
	}
	if client, ok := otherMap["client_software"].(string); ok && strings.TrimSpace(client) != "" {
		return strings.TrimSpace(client)
	}
	headers, ok := otherMap["request_headers"].(map[string]interface{})
	if !ok {
		return ""
	}
	for _, key := range []string{"User-Agent", "X-Title", "HTTP-Referer", "Referer", "Origin", "X-Client", "X-Client-Name", "X-SillyTavern-Version"} {
		for headerKey, value := range headers {
			if strings.EqualFold(headerKey, key) {
				if s, ok := value.(string); ok && strings.TrimSpace(s) != "" {
					return strings.TrimSpace(s)
				}
			}
		}
	}
	return ""
}

func mostCommonString(values map[string]int) string {
	best := ""
	bestCount := 0
	for value, count := range values {
		if count > bestCount || (count == bestCount && value < best) {
			best = value
			bestCount = count
		}
	}
	return best
}

const logSearchCountLimit = 10000

func GetUserLogs(userId int, logType int, startTimestamp int64, endTimestamp int64, modelName string, tokenName string, startIdx int, num int, group string, requestId string) (logs []*Log, total int64, err error) {
	var tx *gorm.DB
	if logType == LogTypeUnknown {
		tx = LOG_DB.Where("logs.user_id = ?", userId)
	} else {
		tx = LOG_DB.Where("logs.user_id = ? and logs.type = ?", userId, logType)
	}

	if modelName != "" {
		modelNamePattern, err := sanitizeLikePattern(modelName)
		if err != nil {
			return nil, 0, err
		}
		tx = tx.Where("logs.model_name LIKE ? ESCAPE '!'", modelNamePattern)
	}
	if tokenName != "" {
		tx = tx.Where("logs.token_name = ?", tokenName)
	}
	if requestId != "" {
		tx = tx.Where("logs.request_id = ?", requestId)
	}
	if startTimestamp != 0 {
		tx = tx.Where("logs.created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("logs.created_at <= ?", endTimestamp)
	}
	if group != "" {
		tx = tx.Where("logs."+logGroupCol+" = ?", group)
	}
	err = tx.Model(&Log{}).Limit(logSearchCountLimit).Count(&total).Error
	if err != nil {
		common.SysError("failed to count user logs: " + err.Error())
		return nil, 0, errors.New("查询日志失败")
	}
	err = tx.Order("logs.id desc").Limit(num).Offset(startIdx).Find(&logs).Error
	if err != nil {
		common.SysError("failed to search user logs: " + err.Error())
		return nil, 0, errors.New("查询日志失败")
	}

	formatUserLogs(logs, startIdx)
	return logs, total, err
}

type Stat struct {
	Quota int `json:"quota"`
	Rpm   int `json:"rpm"`
	Tpm   int `json:"tpm"`
}

func SumUsedQuota(logType int, startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string, channel int, group string) (stat Stat, err error) {
	tx := LOG_DB.Table("logs").Select("sum(quota) quota")

	// 为rpm和tpm创建单独的查询
	rpmTpmQuery := LOG_DB.Table("logs").Select("count(*) rpm, sum(prompt_tokens) + sum(completion_tokens) tpm")

	if username != "" {
		tx = tx.Where("username = ?", username)
		rpmTpmQuery = rpmTpmQuery.Where("username = ?", username)
	}
	if tokenName != "" {
		tx = tx.Where("token_name = ?", tokenName)
		rpmTpmQuery = rpmTpmQuery.Where("token_name = ?", tokenName)
	}
	if startTimestamp != 0 {
		tx = tx.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("created_at <= ?", endTimestamp)
	}
	if modelName != "" {
		modelNamePattern, err := sanitizeLikePattern(modelName)
		if err != nil {
			return stat, err
		}
		tx = tx.Where("model_name LIKE ? ESCAPE '!'", modelNamePattern)
		rpmTpmQuery = rpmTpmQuery.Where("model_name LIKE ? ESCAPE '!'", modelNamePattern)
	}
	if channel != 0 {
		tx = tx.Where("channel_id = ?", channel)
		rpmTpmQuery = rpmTpmQuery.Where("channel_id = ?", channel)
	}
	if group != "" {
		tx = tx.Where(logGroupCol+" = ?", group)
		rpmTpmQuery = rpmTpmQuery.Where(logGroupCol+" = ?", group)
	}

	tx = tx.Where("type = ?", LogTypeConsume)
	rpmTpmQuery = rpmTpmQuery.Where("type = ?", LogTypeConsume)

	// 只统计最近60秒的rpm和tpm
	rpmTpmQuery = rpmTpmQuery.Where("created_at >= ?", time.Now().Add(-60*time.Second).Unix())

	// 执行查询
	if err := tx.Scan(&stat).Error; err != nil {
		common.SysError("failed to query log stat: " + err.Error())
		return stat, errors.New("查询统计数据失败")
	}
	if err := rpmTpmQuery.Scan(&stat).Error; err != nil {
		common.SysError("failed to query rpm/tpm stat: " + err.Error())
		return stat, errors.New("查询统计数据失败")
	}

	return stat, nil
}

func SumUsedToken(logType int, startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string) (token int) {
	tx := LOG_DB.Table("logs").Select("ifnull(sum(prompt_tokens),0) + ifnull(sum(completion_tokens),0)")
	if username != "" {
		tx = tx.Where("username = ?", username)
	}
	if tokenName != "" {
		tx = tx.Where("token_name = ?", tokenName)
	}
	if startTimestamp != 0 {
		tx = tx.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("created_at <= ?", endTimestamp)
	}
	if modelName != "" {
		tx = tx.Where("model_name = ?", modelName)
	}
	tx.Where("type = ?", LogTypeConsume).Scan(&token)
	return token
}

func DeleteOldLog(ctx context.Context, targetTimestamp int64, limit int) (int64, error) {
	var total int64 = 0

	for {
		if nil != ctx.Err() {
			return total, ctx.Err()
		}

		result := LOG_DB.Where("created_at < ?", targetTimestamp).Limit(limit).Delete(&Log{})
		if nil != result.Error {
			return total, result.Error
		}

		total += result.RowsAffected

		if result.RowsAffected < int64(limit) {
			break
		}
	}

	return total, nil
}
