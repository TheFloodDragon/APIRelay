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
