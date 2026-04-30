package controller

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

const maxSQLiteImportBytes = 1024 << 20

func DownloadSQLiteDatabaseBackup(c *gin.Context) {
	if !common.UsingSQLite {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "当前主数据库不是 SQLite，无法使用 SQLite 数据库导出",
		})
		return
	}

	dbPath, err := getSQLiteDatabasePath()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if info, err := os.Stat(dbPath); err != nil || info.IsDir() {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "SQLite 数据库文件不存在或不可读",
		})
		return
	}

	backupDir := filepath.Join(os.TempDir(), "new-api-db-backups")
	if err := os.MkdirAll(backupDir, 0700); err != nil {
		common.ApiError(c, err)
		return
	}
	backupPath := filepath.Join(backupDir, fmt.Sprintf("new-api-sqlite-backup-%s.db", time.Now().Format("20060102-150405")))
	if err := createSQLiteBackup(backupPath); err != nil {
		_ = os.Remove(backupPath)
		common.ApiError(c, err)
		return
	}
	defer func() {
		_ = os.Remove(backupPath)
	}()

	model.RecordLog(c.GetInt("id"), model.LogTypeSystem, fmt.Sprintf("超级管理员导出 SQLite 数据库备份，IP：%s", c.ClientIP()))

	c.Header("Cache-Control", "no-store, no-cache, must-revalidate, private")
	c.Header("Pragma", "no-cache")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Description", "File Transfer")
	c.FileAttachment(backupPath, filepath.Base(backupPath))
}

func ImportSQLiteDatabaseBackup(c *gin.Context) {
	if !common.UsingSQLite {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "当前主数据库不是 SQLite，无法导入 SQLite 数据库备份",
		})
		return
	}

	dbPath, err := getSQLiteDatabasePath()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if info, err := os.Stat(dbPath); err != nil || info.IsDir() {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "当前 SQLite 数据库文件不存在或不可读",
		})
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxSQLiteImportBytes)
	fileHeader, err := c.FormFile("database")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "请选择要导入的 SQLite 数据库文件",
		})
		return
	}
	if fileHeader.Size <= 0 || fileHeader.Size > maxSQLiteImportBytes {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "数据库文件大小异常或超过 1024MB",
		})
		return
	}

	tmpDir := filepath.Join(os.TempDir(), "new-api-db-imports")
	if err := os.MkdirAll(tmpDir, 0700); err != nil {
		common.ApiError(c, err)
		return
	}
	importPath := filepath.Join(tmpDir, fmt.Sprintf("new-api-sqlite-import-%s.db", time.Now().Format("20060102-150405")))
	if err := saveUploadedFile(fileHeader, importPath); err != nil {
		_ = os.Remove(importPath)
		common.ApiError(c, err)
		return
	}

	if err := validateSQLiteBackup(importPath); err != nil {
		_ = os.Remove(importPath)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "导入文件不是有效的 New API SQLite 数据库备份：" + err.Error(),
		})
		return
	}

	currentBackupPath := dbPath + fmt.Sprintf(".before-import-%s.bak", time.Now().Format("20060102-150405"))
	model.RecordLog(c.GetInt("id"), model.LogTypeSystem, fmt.Sprintf("超级管理员导入 SQLite 数据库备份，IP：%s", c.ClientIP()))

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "数据库备份已上传并校验通过，服务即将停止以替换数据库。请稍后手动重启服务。",
	})

	go replaceSQLiteDatabaseAndExit(importPath, dbPath, currentBackupPath)
}

func getSQLiteDatabasePath() (string, error) {
	dsn := strings.TrimSpace(common.SQLitePath)
	if dsn == "" {
		return "", fmt.Errorf("SQLite 数据库路径为空")
	}
	if idx := strings.Index(dsn, "?"); idx >= 0 {
		dsn = dsn[:idx]
	}
	if dsn == "" || dsn == ":memory:" || strings.HasPrefix(dsn, "file::memory:") {
		return "", fmt.Errorf("当前 SQLite 数据库不是文件数据库，无法导出")
	}
	absPath, err := filepath.Abs(filepath.Clean(dsn))
	if err != nil {
		return "", err
	}
	return absPath, nil
}

func createSQLiteBackup(backupPath string) error {
	backupSQLPath := strings.ReplaceAll(backupPath, "'", "''")
	return model.DB.Exec("VACUUM INTO '" + backupSQLPath + "'").Error
}

func saveUploadedFile(fileHeader *multipart.FileHeader, targetPath string) error {
	src, err := fileHeader.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	return err
}

func validateSQLiteBackup(path string) error {
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		return err
	}
	sqlDB, err := db.DB()
	if err == nil {
		defer sqlDB.Close()
	}

	var checkResult string
	if err := db.Raw("PRAGMA quick_check").Scan(&checkResult).Error; err != nil {
		return err
	}
	if checkResult != "ok" {
		return fmt.Errorf("SQLite quick_check failed: %s", checkResult)
	}

	var tableCount int64
	if err := db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name IN ('users', 'options')").Scan(&tableCount).Error; err != nil {
		return err
	}
	if tableCount < 2 {
		return fmt.Errorf("缺少 users 或 options 表")
	}
	return nil
}

func replaceSQLiteDatabaseAndExit(importPath string, dbPath string, currentBackupPath string) {
	time.Sleep(800 * time.Millisecond)
	closeGormDB(model.LOG_DB)
	closeGormDB(model.DB)

	if err := os.Rename(dbPath, currentBackupPath); err != nil {
		common.SysError("failed to backup current sqlite database before import: " + err.Error())
		_ = os.Remove(importPath)
		os.Exit(1)
	}
	if err := os.Rename(importPath, dbPath); err != nil {
		common.SysError("failed to replace sqlite database: " + err.Error())
		_ = os.Rename(currentBackupPath, dbPath)
		_ = os.Remove(importPath)
		os.Exit(1)
	}

	common.SysLog("SQLite database import completed. Previous database backup: " + currentBackupPath)
	os.Exit(0)
}

func closeGormDB(db *gorm.DB) {
	if db == nil {
		return
	}
	sqlDB, err := db.DB()
	if err == nil {
		_ = sqlDB.Close()
	}
}
