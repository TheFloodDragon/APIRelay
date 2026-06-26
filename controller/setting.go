package controller

import (
	"encoding/json"
	"net/http"

	"github.com/apirelay/apirelay/constant"
	"github.com/apirelay/apirelay/model"

	"github.com/gin-gonic/gin"
)

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
	if err := c.ShouldBindJSON(&rules); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
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
	if err := c.ShouldBindJSON(&prices); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
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
