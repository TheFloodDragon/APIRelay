package model

import (
	"testing"

	"github.com/apirelay/apirelay/common/config"
)

func setupAbilityTestDB(t *testing.T) {
	t.Helper()
	if err := InitDB(&config.DatabaseConfig{Driver: "sqlite", DSN: "file::memory:?cache=shared"}); err != nil {
		t.Fatalf("init db: %v", err)
	}
	DB.Exec("DELETE FROM channels")
	DB.Exec("DELETE FROM abilities")
}

func TestDiagnoseModel_EnabledProvider(t *testing.T) {
	setupAbilityTestDB(t)
	ch := &Channel{Name: "openai-main", Type: 1, Status: ChannelStatusEnabled, Group: "default",
		ModelConfigs: `[{"name":"gpt-4o","enabled":true}]`}
	if err := CreateChannel(ch); err != nil {
		t.Fatalf("create: %v", err)
	}
	diag := DiagnoseModel("default", "gpt-4o")
	if len(diag.EnabledProviders) != 1 || diag.EnabledProviders[0] != "openai-main" {
		t.Errorf("enabled providers = %v, want [openai-main]", diag.EnabledProviders)
	}
}

func TestDiagnoseModel_GroupMismatch(t *testing.T) {
	setupAbilityTestDB(t)
	ch := &Channel{Name: "vip-channel", Type: 1, Status: ChannelStatusEnabled, Group: "vip",
		ModelConfigs: `[{"name":"claude-opus","enabled":true}]`}
	CreateChannel(ch)
	// 在 default 分组下请求，应诊断为分组不匹配
	diag := DiagnoseModel("default", "claude-opus")
	if len(diag.EnabledProviders) != 0 {
		t.Errorf("should have no enabled providers in default, got %v", diag.EnabledProviders)
	}
	if len(diag.OtherGroupProviders) != 1 || diag.OtherGroupProviders[0] != "vip-channel" {
		t.Errorf("other-group providers = %v, want [vip-channel]", diag.OtherGroupProviders)
	}
}

func TestDiagnoseModel_Disabled(t *testing.T) {
	setupAbilityTestDB(t)
	ch := &Channel{Name: "off-channel", Type: 1, Status: ChannelStatusDisabled, Group: "default",
		ModelConfigs: `[{"name":"gpt-4o","enabled":true}]`}
	CreateChannel(ch)
	diag := DiagnoseModel("default", "gpt-4o")
	if len(diag.EnabledProviders) != 0 {
		t.Errorf("disabled channel should not be enabled-provider, got %v", diag.EnabledProviders)
	}
	if len(diag.DisabledProviders) != 1 || diag.DisabledProviders[0] != "off-channel" {
		t.Errorf("disabled providers = %v, want [off-channel]", diag.DisabledProviders)
	}
}

func TestDiagnoseModel_Wildcard(t *testing.T) {
	setupAbilityTestDB(t)
	ch := &Channel{Name: "wild", Type: 1, Status: ChannelStatusEnabled, Group: "default",
		ModelConfigs: `[{"name":"*","enabled":true}]`}
	CreateChannel(ch)
	diag := DiagnoseModel("default", "any-model")
	if !diag.HasWildcard {
		t.Error("should detect wildcard channel")
	}
	if len(diag.EnabledProviders) != 1 {
		t.Errorf("wildcard should count as enabled provider, got %v", diag.EnabledProviders)
	}
}

func TestCreateChannelSyncsAbilities(t *testing.T) {
	setupAbilityTestDB(t)
	ch := &Channel{
		Name:     "sync-create",
		Type:     1,
		Status:   ChannelStatusEnabled,
		Group:    "vip",
		Priority: 7,
		Weight:   3,
		ModelConfigs: `[
			{"name":"gpt-4o","enabled":true},
			{"name":"disabled-model","enabled":false}
		]`,
	}
	if err := CreateChannel(ch); err != nil {
		t.Fatalf("create channel: %v", err)
	}

	var abilities []Ability
	if err := DB.Where("channel_id = ?", ch.Id).Find(&abilities).Error; err != nil {
		t.Fatalf("query abilities: %v", err)
	}
	if len(abilities) != 1 {
		t.Fatalf("abilities = %d, want 1: %+v", len(abilities), abilities)
	}
	a := abilities[0]
	if a.Group != "vip" || a.Model != "gpt-4o" || !a.Enabled || a.Priority != 7 || a.Weight != 3 {
		t.Fatalf("ability not synced from channel: %+v", a)
	}
}

func TestUpdateChannelReplacesAbilities(t *testing.T) {
	setupAbilityTestDB(t)
	ch := &Channel{Name: "sync-update", Type: 1, Status: ChannelStatusEnabled, Group: "default",
		ModelConfigs: `[{"name":"old-a","enabled":true},{"name":"old-b","enabled":true}]`}
	if err := CreateChannel(ch); err != nil {
		t.Fatalf("create channel: %v", err)
	}

	ch.ModelConfigs = `[{"name":"new-a","enabled":true},{"name":"new-b","enabled":true},{"name":"old-a","enabled":false}]`
	if err := UpdateChannel(ch); err != nil {
		t.Fatalf("update channel: %v", err)
	}

	var abilities []Ability
	if err := DB.Where("channel_id = ?", ch.Id).Order("model asc").Find(&abilities).Error; err != nil {
		t.Fatalf("query abilities: %v", err)
	}
	if len(abilities) != 2 {
		t.Fatalf("abilities = %d, want 2: %+v", len(abilities), abilities)
	}
	if abilities[0].Model != "new-a" || abilities[1].Model != "new-b" {
		t.Fatalf("abilities not replaced: %+v", abilities)
	}
}

func TestDeleteChannelDeletesAbilities(t *testing.T) {
	setupAbilityTestDB(t)
	ch := &Channel{Name: "sync-delete", Type: 1, Status: ChannelStatusEnabled, Group: "default",
		ModelConfigs: `[{"name":"gpt-4o","enabled":true}]`}
	if err := CreateChannel(ch); err != nil {
		t.Fatalf("create channel: %v", err)
	}
	if err := DeleteChannel(ch.Id); err != nil {
		t.Fatalf("delete channel: %v", err)
	}

	var abilityCount int64
	if err := DB.Model(&Ability{}).Where("channel_id = ?", ch.Id).Count(&abilityCount).Error; err != nil {
		t.Fatalf("count abilities: %v", err)
	}
	if abilityCount != 0 {
		t.Fatalf("abilities after delete = %d, want 0", abilityCount)
	}
	var channelCount int64
	if err := DB.Model(&Channel{}).Where("id = ?", ch.Id).Count(&channelCount).Error; err != nil {
		t.Fatalf("count channels: %v", err)
	}
	if channelCount != 0 {
		t.Fatalf("channels after delete = %d, want 0", channelCount)
	}
}
