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
	var in model.Token
	if err := c.ShouldBindJSON(&in); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	in.UserId = currentUserID(c)
	if in.Status == 0 {
		in.Status = model.TokenStatusEnabled
	}
	if in.Group == "" {
		in.Group = "default"
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
