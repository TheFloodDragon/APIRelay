package controller

import "github.com/gin-gonic/gin"

// 统一响应封装。
func ok(c *gin.Context, data any) {
	c.JSON(200, gin.H{"success": true, "data": data})
}

func fail(c *gin.Context, status int, msg string) {
	c.JSON(status, gin.H{"success": false, "message": msg})
}
