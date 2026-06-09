package service

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/TheFloodDragon/APIRelay/internal/repository"
	"gorm.io/gorm"
)

const (
	settingKeyModelTestDefaultPrompt         = "model_test.default_prompt"
	settingKeyModelTestTimeoutMS             = "model_test.timeout_ms"
	settingKeyModelTestMaxOutputTokens       = "model_test.max_output_tokens"
	settingKeyModelTestTemperature           = "model_test.temperature"
	settingKeyModelTestIncludeDisabledModels = "model_test.include_disabled_models"
)

// Settings 是管理台全局设置的统一响应结构。
type Settings struct {
	ModelTest ModelTestSettings `json:"model_test"`
}

// ModelTestSettings 仅影响管理台模型测试，不影响真实请求路由。
type ModelTestSettings struct {
	DefaultPrompt         string  `json:"default_prompt"`
	TimeoutMS             int     `json:"timeout_ms"`
	MaxOutputTokens       int     `json:"max_output_tokens"`
	Temperature           float64 `json:"temperature"`
	IncludeDisabledModels bool    `json:"include_disabled_models"`
}

func DefaultSettings() Settings {
	return Settings{ModelTest: DefaultModelTestSettings()}
}

func DefaultModelTestSettings() ModelTestSettings {
	return ModelTestSettings{
		DefaultPrompt:         "Say OK in one short sentence.",
		TimeoutMS:             30000,
		MaxOutputTokens:       32,
		Temperature:           0,
		IncludeDisabledModels: true,
	}
}

type SettingsService struct {
	repo *repository.SystemConfigRepository
}

func NewSettingsService(repo *repository.SystemConfigRepository) *SettingsService {
	return &SettingsService{repo: repo}
}

func (s *SettingsService) GetSettings() (Settings, error) {
	settings := DefaultSettings()
	if s == nil || s.repo == nil {
		return settings, nil
	}

	settings.ModelTest.DefaultPrompt = s.getString(settingKeyModelTestDefaultPrompt, settings.ModelTest.DefaultPrompt)
	settings.ModelTest.TimeoutMS = s.getInt(settingKeyModelTestTimeoutMS, settings.ModelTest.TimeoutMS)
	settings.ModelTest.MaxOutputTokens = s.getInt(settingKeyModelTestMaxOutputTokens, settings.ModelTest.MaxOutputTokens)
	settings.ModelTest.Temperature = s.getFloat(settingKeyModelTestTemperature, settings.ModelTest.Temperature)
	settings.ModelTest.IncludeDisabledModels = s.getBool(settingKeyModelTestIncludeDisabledModels, settings.ModelTest.IncludeDisabledModels)
	settings.ModelTest = sanitizeModelTestSettings(settings.ModelTest)
	return settings, nil
}

func (s *SettingsService) UpdateSettings(settings Settings) (Settings, error) {
	if s == nil || s.repo == nil {
		return DefaultSettings(), fmt.Errorf("settings repository is not configured")
	}
	settings.ModelTest = sanitizeModelTestSettings(settings.ModelTest)

	pairs := map[string]interface{}{
		settingKeyModelTestDefaultPrompt:         settings.ModelTest.DefaultPrompt,
		settingKeyModelTestTimeoutMS:             settings.ModelTest.TimeoutMS,
		settingKeyModelTestMaxOutputTokens:       settings.ModelTest.MaxOutputTokens,
		settingKeyModelTestTemperature:           settings.ModelTest.Temperature,
		settingKeyModelTestIncludeDisabledModels: settings.ModelTest.IncludeDisabledModels,
	}
	for key, value := range pairs {
		encoded, err := json.Marshal(value)
		if err != nil {
			return settings, err
		}
		if err := s.repo.Set(key, string(encoded)); err != nil {
			return settings, err
		}
	}
	return settings, nil
}

func sanitizeModelTestSettings(settings ModelTestSettings) ModelTestSettings {
	defaults := DefaultModelTestSettings()
	if settings.DefaultPrompt == "" {
		settings.DefaultPrompt = defaults.DefaultPrompt
	}
	if settings.TimeoutMS <= 0 {
		settings.TimeoutMS = defaults.TimeoutMS
	}
	if settings.TimeoutMS > 600000 {
		settings.TimeoutMS = 600000
	}
	if settings.MaxOutputTokens <= 0 {
		settings.MaxOutputTokens = defaults.MaxOutputTokens
	}
	if settings.MaxOutputTokens > 8192 {
		settings.MaxOutputTokens = 8192
	}
	if settings.Temperature < 0 {
		settings.Temperature = 0
	}
	if settings.Temperature > 2 {
		settings.Temperature = 2
	}
	return settings
}

func (s *SettingsService) getRaw(key string) (string, bool) {
	value, err := s.repo.Get(key)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", false
		}
		return "", false
	}
	return value, true
}

func (s *SettingsService) getString(key, fallback string) string {
	value, ok := s.getRaw(key)
	if !ok {
		return fallback
	}
	var decoded string
	if err := json.Unmarshal([]byte(value), &decoded); err == nil {
		return decoded
	}
	if value != "" {
		return value
	}
	return fallback
}

func (s *SettingsService) getInt(key string, fallback int) int {
	value, ok := s.getRaw(key)
	if !ok {
		return fallback
	}
	var decoded int
	if err := json.Unmarshal([]byte(value), &decoded); err == nil {
		return decoded
	}
	return fallback
}

func (s *SettingsService) getFloat(key string, fallback float64) float64 {
	value, ok := s.getRaw(key)
	if !ok {
		return fallback
	}
	var decoded float64
	if err := json.Unmarshal([]byte(value), &decoded); err == nil {
		return decoded
	}
	return fallback
}

func (s *SettingsService) getBool(key string, fallback bool) bool {
	value, ok := s.getRaw(key)
	if !ok {
		return fallback
	}
	var decoded bool
	if err := json.Unmarshal([]byte(value), &decoded); err == nil {
		return decoded
	}
	return fallback
}
