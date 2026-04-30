package model

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
)

const FakeModelsOptionKey = "global.fake_models"
const FakeModelResponseOptionKey = "global.fake_model_response"

var defaultFakeModels = []string{"gpt10", "deepseekv10", "claude opus10"}

func GetFakeModels() []string {
	common.OptionMapRWMutex.RLock()
	value := common.OptionMap[FakeModelsOptionKey]
	common.OptionMapRWMutex.RUnlock()

	models := parseFakeModels(value)
	if len(models) == 0 {
		return defaultFakeModels
	}
	return models
}

func IsFakeModel(modelName string) bool {
	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		return false
	}
	for _, fakeModel := range GetFakeModels() {
		if strings.EqualFold(modelName, strings.TrimSpace(fakeModel)) {
			return true
		}
	}
	return false
}

func GetFakeModelResponse() string {
	common.OptionMapRWMutex.RLock()
	value := strings.TrimSpace(common.OptionMap[FakeModelResponseOptionKey])
	common.OptionMapRWMutex.RUnlock()
	if value == "" {
		return "baka"
	}
	return value
}

func parseFakeModels(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	var items []string
	if strings.HasPrefix(value, "[") {
		_ = common.Unmarshal([]byte(value), &items)
	} else {
		items = strings.Split(value, ",")
	}

	seen := map[string]bool{}
	models := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		key := strings.ToLower(item)
		if seen[key] {
			continue
		}
		seen[key] = true
		models = append(models, item)
	}
	return models
}
