package controller

import (
	"net/http"
	"strconv"

	"github.com/apirelay/apirelay/model"

	"github.com/gin-gonic/gin"
)

// ListChannels GET /api/channels
func ListChannels(c *gin.Context) {
	list, err := model.ListChannels()
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, list)
}

// CreateChannel POST /api/channels
func CreateChannel(c *gin.Context) {
	var ch model.Channel
	if err := c.ShouldBindJSON(&ch); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if ch.Status == 0 {
		ch.Status = model.ChannelStatusEnabled
	}
	if ch.Group == "" {
		ch.Group = "default"
	}
	if ch.Weight == 0 {
		ch.Weight = 1
	}
	if err := model.CreateChannel(&ch); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, ch)
}

// UpdateChannel PUT /api/channels/:id
func UpdateChannel(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	existing, err := model.GetChannelByID(id)
	if err != nil {
		fail(c, http.StatusNotFound, "channel not found")
		return
	}
	var in model.Channel
	if err := c.ShouldBindJSON(&in); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	in.Id = existing.Id
	in.CreatedAt = existing.CreatedAt
	if err := model.UpdateChannel(&in); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, in)
}

// DeleteChannel DELETE /api/channels/:id
func DeleteChannel(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := model.DeleteChannel(id); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, gin.H{"deleted": id})
}
