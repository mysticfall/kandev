package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/kandev/kandev/internal/agent/managedruntime"
)

func (s *Server) handleOpenCodeCredentials(c *gin.Context) {
	data, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20))
	var entries []json.RawMessage
	if err != nil || json.Unmarshal(data, &entries) != nil || len(entries) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid OpenCode credential payload"})
		return
	}
	if err := s.procMgr.ImportOpenCodeCredentials(c.Request.Context(), data); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, managedruntime.ErrOpenCodeCredentialRuntime) {
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}
