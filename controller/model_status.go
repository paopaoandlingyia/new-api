package controller

import (
	"errors"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func GetPublishedGroupStatuses(c *gin.Context) {
	userGroup := ""
	if userID, exists := c.Get("id"); exists {
		user, err := model.GetUserCache(userID.(int))
		if err == nil {
			userGroup = user.Group
		}
	}
	statuses, err := service.GetPublishedGroupStatuses(service.GetUserUsableGroups(userGroup))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, statuses)
}

func GetPublishedGroupAvailability(c *gin.Context) {
	userGroup := ""
	if userID, exists := c.Get("id"); exists {
		user, err := model.GetUserCache(userID.(int))
		if err == nil {
			userGroup = user.Group
		}
	}
	summary, err := service.GetPublishedGroupAvailability(service.GetUserUsableGroups(userGroup))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, summary)
}

func GetManagedGroupStatuses(c *gin.Context) {
	statuses, err := service.GetManagedGroupStatuses()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, statuses)
}

func GetModelStatusSources(c *gin.Context) {
	sources, err := service.GetModelStatusSources()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, sources)
}

func CreateModelStatusSource(c *gin.Context) {
	var input service.ModelStatusSourceInput
	if err := common.DecodeJson(c.Request.Body, &input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid request payload"})
		return
	}
	if err := service.CreateModelStatusSource(input); err != nil {
		writeModelStatusSourceError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func UpdateModelStatusSource(c *gin.Context) {
	var input service.ModelStatusSourceInput
	if err := common.DecodeJson(c.Request.Body, &input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid request payload"})
		return
	}
	if err := service.UpdateModelStatusSource(c.Param("id"), input); err != nil {
		writeModelStatusSourceError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func DeleteModelStatusSource(c *gin.Context) {
	if err := service.DeleteModelStatusSource(c.Param("id")); err != nil {
		writeModelStatusSourceError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func writeModelStatusSourceError(c *gin.Context, err error) {
	if errors.Is(err, service.ErrInvalidModelStatusSource) {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
}

func UpdateManagedGroupStatus(c *gin.Context) {
	var update service.GroupStatusUpdate
	if err := common.DecodeJson(c.Request.Body, &update); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid request payload"})
		return
	}
	if err := service.UpdateGroupStatus(update); err != nil {
		if errors.Is(err, service.ErrInvalidGroupStatusUpdate) {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	common.ApiSuccess(c, nil)
}
