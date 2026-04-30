package controller

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
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

	tmpDir := filepath.Join(filepath.Dir(dbPath), ".new-api-db-imports")
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

	pendingPath := pendingSQLiteImportPath(dbPath)
	_ = os.Remove(pendingPath)
	if err := moveFile(importPath, pendingPath); err != nil {
		_ = os.Remove(importPath)
		common.ApiError(c, err)
		return
	}
	model.RecordLog(c.GetInt("id"), model.LogTypeSystem, fmt.Sprintf("超级管理员导入 SQLite 数据库备份，IP：%s", c.ClientIP()))

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "数据库备份已上传并校验通过。请点击重启 New API 以应用导入。",
		"data": gin.H{
			"pending_import": true,
		},
	})
}

func RestartNewAPI(c *gin.Context) {
	dbPath, err := getSQLiteDatabasePath()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pendingPath := pendingSQLiteImportPath(dbPath)
	hasPendingImport := false
	if info, err := os.Stat(pendingPath); err == nil && !info.IsDir() {
		hasPendingImport = true
	}

	model.RecordLog(c.GetInt("id"), model.LogTypeSystem, fmt.Sprintf("超级管理员请求重启 New API，IP：%s，待应用导入：%t", c.ClientIP(), hasPendingImport))

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "服务即将重启。若有待导入数据库，将在退出前完成替换。",
		"data":    gin.H{"pending_import": hasPendingImport},
	})

	go restartNewAPIAfterResponse(dbPath, pendingPath, hasPendingImport)
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

func pendingSQLiteImportPath(dbPath string) string {
	return dbPath + ".pending-import"
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

func restartNewAPIAfterResponse(dbPath string, pendingPath string, hasPendingImport bool) {
	time.Sleep(800 * time.Millisecond)
	closeGormDB(model.LOG_DB)
	closeGormDB(model.DB)
	if hasPendingImport {
		currentBackupPath := dbPath + fmt.Sprintf(".before-import-%s.bak", time.Now().Format("20060102-150405"))
		if err := replaceSQLiteDatabase(pendingPath, dbPath, currentBackupPath); err != nil {
			common.SysError("failed to apply pending sqlite import: " + err.Error())
			os.Exit(1)
		}
		common.SysLog("SQLite database import completed. Previous database backup: " + currentBackupPath)
	}
	if err := startNewAPIProcess(); err != nil {
		common.SysError("failed to start replacement New API process: " + err.Error())
		os.Exit(1)
	}
	os.Exit(0)
}

func startNewAPIProcess() error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	workingDir, err := os.Getwd()
	if err != nil {
		return err
	}

	if runtime.GOOS == "windows" {
		cmdPath := filepath.Join(os.TempDir(), fmt.Sprintf("new-api-restart-%d.cmd", time.Now().UnixNano()))
		quotedArgs := make([]string, 0, len(os.Args))
		quotedArgs = append(quotedArgs, strconv.Quote(executable))
		for _, arg := range os.Args[1:] {
			quotedArgs = append(quotedArgs, strconv.Quote(arg))
		}
		cmdContent := fmt.Sprintf(
			"@echo off\r\ncd /d %s\r\ntimeout /t 2 /nobreak >nul\r\nstart \"\" %s\r\ndel \"%%~f0\"\r\n",
			strconv.Quote(workingDir),
			strings.Join(quotedArgs, " "),
		)
		if err := os.WriteFile(cmdPath, []byte(cmdContent), 0600); err != nil {
			return err
		}
		cmd := exec.Command("cmd", "/C", "start", "", cmdPath)
		return cmd.Start()
	}

	args := append([]string{executable}, os.Args[1:]...)
	if err := syscall.Exec(executable, args, os.Environ()); err != nil {
		common.SysError("syscall exec restart failed, fallback to shell start: " + err.Error())
	}
	quotedArgs := make([]string, 0, len(args))
	for _, arg := range args {
		quotedArgs = append(quotedArgs, shellQuote(arg))
	}
	script := fmt.Sprintf("sleep 1; cd %s && exec %s", shellQuote(workingDir), strings.Join(quotedArgs, " "))
	cmd := exec.Command("sh", "-c", script)
	cmd.Env = os.Environ()
	return cmd.Start()
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func replaceSQLiteDatabase(importPath string, dbPath string, currentBackupPath string) error {
	if err := os.Rename(dbPath, currentBackupPath); err != nil {
		return fmt.Errorf("failed to backup current sqlite database before import: %w", err)
	}
	if err := moveFile(importPath, dbPath); err != nil {
		_ = os.Rename(currentBackupPath, dbPath)
		return fmt.Errorf("failed to replace sqlite database: %w", err)
	}
	return nil
}

func moveFile(src string, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	input, err := os.Open(src)
	if err != nil {
		return err
	}
	defer input.Close()

	output, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	if _, err = io.Copy(output, input); err != nil {
		_ = output.Close()
		_ = os.Remove(dst)
		return err
	}
	if err = output.Close(); err != nil {
		_ = os.Remove(dst)
		return err
	}
	return os.Remove(src)
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
