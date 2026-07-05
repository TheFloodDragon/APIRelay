package controller

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"

	"github.com/apirelay/apirelay/common/config"
	"github.com/apirelay/apirelay/constant"
	"github.com/apirelay/apirelay/model"

	"github.com/gin-gonic/gin"
)

type configFileResponse struct {
	Path    string `json:"path"`
	Exists  bool   `json:"exists"`
	Content string `json:"content"`
	Message string `json:"message,omitempty"`
}

type updateConfigFileRequest struct {
	Content string `json:"content"`
}

// GetConfigFile GET /api/settings/config-file
// 返回当前启动使用的配置文件路径与文件内容。
func GetConfigFile(c *gin.Context) {
	path := config.ConfigFilePath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			ok(c, configFileResponse{Path: path, Exists: false, Content: ""})
			return
		}
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, configFileResponse{Path: path, Exists: true, Content: string(data)})
}

// UpdateConfigFile PUT /api/settings/config-file
// 校验并写回当前启动使用的配置文件。
func UpdateConfigFile(c *gin.Context) {
	var req updateConfigFileRequest
	if !bindJSON(c, &req) {
		return
	}
	if err := config.ValidateYAML([]byte(req.Content)); err != nil {
		fail(c, http.StatusBadRequest, "配置文件 YAML 无法解析: "+err.Error())
		return
	}

	path := config.ConfigFilePath()
	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			fail(c, http.StatusInternalServerError, err.Error())
			return
		}
	}
	if err := os.WriteFile(path, []byte(req.Content), 0o600); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, configFileResponse{
		Path:    path,
		Exists:  true,
		Content: req.Content,
		Message: "配置文件已写入，部分配置需要重启后生效",
	})
}

// GetProtocolRules GET /api/settings/protocol-rules
// 返回全局协议正则规则。
func GetProtocolRules(c *gin.Context) {
	rules := model.GetGlobalProtocolRules()
	if rules == nil {
		rules = []model.ProtocolRule{}
	}
	ok(c, rules)
}

// UpdateProtocolRules PUT /api/settings/protocol-rules
// 保存全局协议正则规则。
func UpdateProtocolRules(c *gin.Context) {
	var rules []model.ProtocolRule
	if !bindJSON(c, &rules) {
		return
	}
	// 校验协议名合法
	for _, r := range rules {
		if r.Protocol != "" {
			if _, hit := constant.APITypeFromName(r.Protocol); !hit {
				fail(c, http.StatusBadRequest, "unknown protocol: "+r.Protocol)
				return
			}
		}
	}
	data, err := json.Marshal(rules)
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if err := model.SetSetting(model.SettingKeyProtocolRules, string(data)); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, rules)
}

// ListProtocols GET /api/protocols
// 返回可选的上游协议列表，供前端下拉。
func ListProtocols(c *gin.Context) {
	ok(c, []gin.H{
		{"value": constant.APINameOpenAI, "name": "OpenAI"},
		{"value": constant.APINameAnthropic, "name": "Anthropic"},
		{"value": constant.APINameResponses, "name": "OpenAI-Responses"},
	})
}

// GetModelPrices GET /api/settings/model-prices
// 返回全局模型价格表（USD / 1M tokens）。
func GetModelPrices(c *gin.Context) {
	prices := model.GetGlobalModelPrices()
	if prices == nil {
		prices = []model.ModelPrice{}
	}
	ok(c, prices)
}

// UpdateModelPrices PUT /api/settings/model-prices
// 保存全局模型价格表。
func UpdateModelPrices(c *gin.Context) {
	var prices []model.ModelPrice
	if !bindJSON(c, &prices) {
		return
	}
	for i := range prices {
		if prices[i].Model == "" {
			fail(c, http.StatusBadRequest, "价格条目缺少模型名（可用 default 作为兜底）")
			return
		}
		if prices[i].Input < 0 || prices[i].Output < 0 {
			fail(c, http.StatusBadRequest, "价格不能为负数")
			return
		}
	}
	data, err := json.Marshal(prices)
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if err := model.SetSetting(model.SettingKeyModelPrices, string(data)); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, prices)
}
