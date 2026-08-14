package controller

import (
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func GetModelAvailability(c *gin.Context) {
	c.JSON(200, gin.H{
		"success": true,
		"data":    service.GetModelAvailability(c.Request.Context()),
	})
}
