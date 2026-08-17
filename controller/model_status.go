package controller

import (
	"errors"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func GetPublishedModelStatuses(c *gin.Context) {
	statuses, err := service.GetPublishedModelStatuses()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, statuses)
}

func GetManagedModelStatuses(c *gin.Context) {
	statuses, err := service.GetManagedModelStatuses()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, statuses)
}

func UpdateManagedModelStatus(c *gin.Context) {
	var update service.ModelStatusUpdate
	if err := common.DecodeJson(c.Request.Body, &update); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid request payload"})
		return
	}
	if err := service.UpdateModelStatus(update); err != nil {
		if errors.Is(err, service.ErrInvalidModelStatusUpdate) {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	common.ApiSuccess(c, nil)
}
