package controller

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

// 统一响应封装。
func ok(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

func fail(c *gin.Context, status int, msg string) {
	c.JSON(status, gin.H{"success": false, "message": msg})
}

func bindJSON(c *gin.Context, obj any) bool {
	if err := c.ShouldBindJSON(obj); err != nil {
		if isRequestBodyTooLarge(err) {
			fail(c, http.StatusRequestEntityTooLarge, "request body too large")
			return false
		}
		fail(c, http.StatusBadRequest, err.Error())
		return false
	}
	return true
}

func isRequestBodyTooLarge(err error) bool {
	var maxBytesErr *http.MaxBytesError
	return errors.As(err, &maxBytesErr)
}
