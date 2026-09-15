package setting

import (
	"fmt"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
)

const ModelRequestConcurrencyLimitGroupOption = "ModelRequestConcurrencyLimitGroup"

var modelRequestConcurrencyLimitGroup = map[string]int{}
var modelRequestConcurrencyLimitMutex sync.RWMutex

func ModelRequestConcurrencyLimitGroup2JSONString() string {
	modelRequestConcurrencyLimitMutex.RLock()
	defer modelRequestConcurrencyLimitMutex.RUnlock()

	jsonBytes, err := common.Marshal(modelRequestConcurrencyLimitGroup)
	if err != nil {
		common.SysError("failed to marshal model request concurrency limits: " + err.Error())
		return "{}"
	}
	return string(jsonBytes)
}

func UpdateModelRequestConcurrencyLimitGroupByJSONString(jsonStr string) error {
	limits, err := parseModelRequestConcurrencyLimitGroup(jsonStr)
	if err != nil {
		return err
	}

	modelRequestConcurrencyLimitMutex.Lock()
	modelRequestConcurrencyLimitGroup = limits
	modelRequestConcurrencyLimitMutex.Unlock()
	return nil
}

func GetModelRequestConcurrencyLimit(group string) (int, bool) {
	modelRequestConcurrencyLimitMutex.RLock()
	defer modelRequestConcurrencyLimitMutex.RUnlock()

	limit, found := modelRequestConcurrencyLimitGroup[group]
	return limit, found
}

func CheckModelRequestConcurrencyLimitGroup(jsonStr string) error {
	_, err := parseModelRequestConcurrencyLimitGroup(jsonStr)
	return err
}

func parseModelRequestConcurrencyLimitGroup(jsonStr string) (map[string]int, error) {
	var limits map[string]int
	if err := common.UnmarshalJsonStr(jsonStr, &limits); err != nil {
		return nil, err
	}
	if limits == nil {
		return nil, fmt.Errorf("model request concurrency limits must be a JSON object")
	}
	for group, limit := range limits {
		if strings.TrimSpace(group) == "" {
			return nil, fmt.Errorf("model request concurrency limit contains an empty group name")
		}
		if limit < 1 {
			return nil, fmt.Errorf("group %s has invalid concurrency limit %d: limit must be at least 1", group, limit)
		}
	}
	return limits, nil
}
