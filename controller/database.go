package controller

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"

	"github.com/gin-gonic/gin"
)

func sqliteDBFilePath() string {
	path := common.SQLitePath
	if idx := strings.Index(path, "?"); idx >= 0 {
		path = path[:idx]
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join("/data", path)
	}
	return path
}

func ExportDatabase(c *gin.Context) {
	if !common.UsingSQLite {
		common.ApiError(c, fmt.Errorf("database export currently supports SQLite only"))
		return
	}
	path := sqliteDBFilePath()
	if _, err := os.Stat(path); err != nil {
		common.ApiError(c, err)
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=new-api-%s.db", time.Now().Format("20060102150405")))
	c.File(path)
}

func ImportDatabase(c *gin.Context) {
	if !common.UsingSQLite {
		common.ApiError(c, fmt.Errorf("database import currently supports SQLite only"))
		return
	}
	file, err := c.FormFile("file")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	path := sqliteDBFilePath()
	backup := fmt.Sprintf("%s.bak.%s", path, time.Now().Format("20060102150405"))
	if _, err := os.Stat(path); err == nil {
		if err := os.Rename(path, backup); err != nil {
			common.ApiError(c, err)
			return
		}
	}
	if err := c.SaveUploadedFile(file, path); err != nil {
		_ = os.Rename(backup, path)
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "database imported, please restart the container to use the new database",
		"data":    gin.H{"backup": backup},
	})
}

func RestartService(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "service restart requested",
	})
	go func() {
		time.Sleep(500 * time.Millisecond)
		os.Exit(0)
	}()
}
