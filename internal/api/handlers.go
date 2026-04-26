package api

import (
	"net/http"
	"trading-bot/internal/domain"

	"github.com/gin-gonic/gin"
)

func (s *Server) statusHandler(c *gin.Context) {
	statuses := s.manager.GetStatus()
	c.JSON(http.StatusOK, statuses)
}

func (s *Server) startHandler(c *gin.Context) {
	var req domain.StartRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if _, err := s.manager.StartTrader(c.Request.Context(), req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "started"})
}

func (s *Server) stopHandler(c *gin.Context) {
	var req struct {
		UserID string `json:"user_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := s.manager.StopTrader(req.UserID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "stopped"})
}
