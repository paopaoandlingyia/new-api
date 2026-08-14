package model_availability_setting

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
)

const SourcesOptionKey = "model_availability_setting.sources"

type Setting struct {
	Enabled bool   `json:"enabled"`
	Sources string `json:"sources"`
}

type Source struct {
	Group string `json:"group"`
	URL   string `json:"url"`
	Token string `json:"token,omitempty"`
}

var availabilitySetting = Setting{Sources: "[]"}

func init() {
	config.GlobalConfig.Register("model_availability_setting", &availabilitySetting)
}

func GetSetting() Setting {
	return availabilitySetting
}

func ParseSources(value string) ([]Source, error) {
	var sources []Source
	if err := common.UnmarshalJsonStr(value, &sources); err != nil {
		return nil, fmt.Errorf("invalid availability sources: %w", err)
	}
	if len(sources) > 20 {
		return nil, fmt.Errorf("availability sources cannot exceed 20 entries")
	}
	for index := range sources {
		sources[index].Group = strings.TrimSpace(sources[index].Group)
		sources[index].URL = strings.TrimSpace(sources[index].URL)
		if sources[index].Group == "" || len(sources[index].Group) > 128 {
			return nil, fmt.Errorf("availability source %d has an invalid group", index+1)
		}
		if len(sources[index].URL) > 2048 || len(sources[index].Token) > 4096 {
			return nil, fmt.Errorf("availability source %d exceeds the size limit", index+1)
		}
		parsed, err := url.ParseRequestURI(sources[index].URL)
		if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return nil, fmt.Errorf("availability source %d must use a valid HTTP(S) URL", index+1)
		}
	}
	return sources, nil
}

func ValidateSources(value string) error {
	_, err := ParseSources(value)
	return err
}
