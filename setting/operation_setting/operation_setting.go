package operation_setting

import (
	"strconv"
	"strings"
)

var DemoSiteEnabled = false
var SelfUseModeEnabled = false

var AutomaticDisableKeywords = []string{
	"Your credit balance is too low",
	"This organization has been disabled.",
	"You exceeded your current quota",
	"Permission denied",
	"The security token included in the request is invalid",
	"Operation not allowed",
	"Your account is not authorized",
}

var AutoBanEnabled = false
var AutoBanHeaderKeywords = []string{"tavo"}
var AutoBanUserWhitelist = []string{}
var HoneypotModels = []string{"claude opus10"}
var HoneypotResponse = "baka"
var SmallRequestSurchargeEnabled = true
var SmallRequestInputTokens = 40
var SmallRequestOutputTokens = 40
var SmallRequestSurchargeUSD = 2.0

func AutomaticDisableKeywordsToString() string {
	return strings.Join(AutomaticDisableKeywords, "\n")
}

func AutomaticDisableKeywordsFromString(s string) {
	AutomaticDisableKeywords = []string{}
	ak := strings.Split(s, "\n")
	for _, k := range ak {
		k = strings.TrimSpace(k)
		k = strings.ToLower(k)
		if k != "" {
			AutomaticDisableKeywords = append(AutomaticDisableKeywords, k)
		}
	}
}

func AutoBanHeaderKeywordsToString() string {
	return strings.Join(AutoBanHeaderKeywords, "\n")
}

func AutoBanHeaderKeywordsFromString(s string) {
	AutoBanHeaderKeywords = []string{}
	for _, k := range strings.Split(s, "\n") {
		k = strings.TrimSpace(strings.ToLower(k))
		if k != "" {
			AutoBanHeaderKeywords = append(AutoBanHeaderKeywords, k)
		}
	}
}

func AutoBanUserWhitelistToString() string {
	return strings.Join(AutoBanUserWhitelist, "\n")
}

func AutoBanUserWhitelistFromString(s string) {
	AutoBanUserWhitelist = []string{}
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r' || r == '\t'
	})
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item != "" {
			AutoBanUserWhitelist = append(AutoBanUserWhitelist, strings.ToLower(item))
		}
	}
}

func IsAutoBanWhitelisted(userId int, identifiers ...string) bool {
	idString := strconv.Itoa(userId)
	for _, item := range AutoBanUserWhitelist {
		item = strings.TrimSpace(strings.ToLower(item))
		if item == "" {
			continue
		}
		if item == idString {
			return true
		}
		for _, identifier := range identifiers {
			if item == strings.TrimSpace(strings.ToLower(identifier)) {
				return true
			}
		}
	}
	return false
}

func HoneypotModelsToString() string {
	return strings.Join(HoneypotModels, "\n")
}

func HoneypotModelsFromString(s string) {
	HoneypotModels = []string{}
	for _, item := range strings.Split(s, "\n") {
		item = strings.TrimSpace(item)
		if item != "" {
			HoneypotModels = append(HoneypotModels, item)
		}
	}
}

func IsHoneypotModel(model string) bool {
	model = strings.TrimSpace(strings.ToLower(model))
	for _, item := range HoneypotModels {
		if strings.TrimSpace(strings.ToLower(item)) == model {
			return true
		}
	}
	return false
}

func HoneypotModelsCopy() []string {
	out := make([]string, 0, len(HoneypotModels))
	out = append(out, HoneypotModels...)
	return out
}
