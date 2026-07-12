package controller

import (
	"net/http"
	"strconv"
	"testing"

	"github.com/apirelay/apirelay/common/config"
	"github.com/apirelay/apirelay/model"
	"github.com/gin-gonic/gin"
)

func TestResetChannelHealthEndpointRejectsMissingChannel(t *testing.T) {
	if err := model.InitDB(&config.DatabaseConfig{Driver: "sqlite", DSN: "file:controller-breaker?mode=memory&cache=shared"}); err != nil {
		t.Fatal(err)
	}
	recorder := performChannelRequest(t, http.MethodPost, "/api/channels/987654/health/reset", "", ResetChannelHealth, gin.Param{Key: "id", Value: "987654"})
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestResetChannelHealthEndpointPersistsClosedAndClearsCooldown(t *testing.T) {
	if err := model.InitDB(&config.DatabaseConfig{Driver: "sqlite", DSN: "file:controller-breaker-success?mode=memory&cache=shared"}); err != nil {
		t.Fatal(err)
	}
	channel := &model.Channel{Name: "controller-reset", Status: model.ChannelStatusEnabled, CooldownUntil: 9999999999999}
	if err := model.DB.Create(channel).Error; err != nil {
		t.Fatal(err)
	}
	if err := model.DB.Create(&model.ChannelHealth{ChannelId: channel.Id, CircuitState: model.CircuitOpen, ConsecutiveFailures: 5}).Error; err != nil {
		t.Fatal(err)
	}

	id := gin.Param{Key: "id", Value: stringID(channel.Id)}
	recorder := performChannelRequest(t, http.MethodPost, "/api/channels/reset/health/reset", "", ResetChannelHealth, id)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var gotChannel model.Channel
	if err := model.DB.First(&gotChannel, channel.Id).Error; err != nil {
		t.Fatal(err)
	}
	health, err := model.GetChannelHealth(channel.Id)
	if err != nil {
		t.Fatal(err)
	}
	if gotChannel.CooldownUntil != 0 || health.CircuitState != model.CircuitClosed {
		t.Fatalf("cooldown=%d health=%+v", gotChannel.CooldownUntil, health)
	}
}

func stringID(id int) string {
	return strconv.Itoa(id)
}
