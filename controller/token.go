package controller

import (
	"net/http"
	"strconv"

	"github.com/apirelay/apirelay/common"
	"github.com/apirelay/apirelay/model"

	"github.com/gin-gonic/gin"
)

// ListTokens GET /api/tokens
func ListTokens(c *gin.Context) {
	// MVP：单管理员，user_id 固定取 1
	list, err := model.ListTokens(currentUserID(c))
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, list)
}

// CreateToken POST /api/tokens
func CreateToken(c *gin.Context) {
	var req struct {
		Name      string  `json:"name"`
		Group     string  `json:"group"`
		Models    string  `json:"models"`
		Unlimited bool    `json:"unlimited"`
		QuotaUSD  float64 `json:"quota_usd"` // 额度（美元），unlimited=false 时生效
		ExpiredAt int64   `json:"expired_at"`
	}
	if !bindJSON(c, &req) {
		return
	}
	if req.Name == "" {
		fail(c, http.StatusBadRequest, "令牌名称不能为空")
		return
	}
	group := req.Group
	if group == "" {
		group = "default"
	}
	in := model.Token{
		UserId:    currentUserID(c),
		Name:      req.Name,
		Group:     group,
		Models:    req.Models,
		Status:    model.TokenStatusEnabled,
		Unlimited: req.Unlimited,
		ExpiredAt: req.ExpiredAt,
	}
	if !req.Unlimited {
		if req.QuotaUSD <= 0 {
			fail(c, http.StatusBadRequest, "限额令牌的额度必须大于 0，或开启不限额")
			return
		}
		// 美元 -> 微美元
		in.Quota = int64(req.QuotaUSD * 1_000_000)
		if in.Quota <= 0 {
			fail(c, http.StatusBadRequest, "限额令牌的额度过小，请提高额度或开启不限额")
			return
		}
	}
	plain := common.NewToken("sk-")
	if err := model.CreateToken(&in, plain); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	// 明文 key 仅此一次返回
	ok(c, gin.H{"token": in, "key": plain})
}

// DeleteToken DELETE /api/tokens/:id
func DeleteToken(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := model.DeleteToken(id, currentUserID(c)); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, gin.H{"deleted": id})
}

func currentUserID(c *gin.Context) int {
	if v, ok := c.Get("user_id"); ok {
		if id, _ := v.(int); id > 0 {
			return id
		}
	}
	return 1
}
