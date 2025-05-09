package objectController

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"jh-oss/internal/apiException"
	"jh-oss/internal/services/objectService"
	"jh-oss/pkg/config"
	"jh-oss/pkg/response"
)

type fileListElement struct {
	Name         string `json:"name"`
	Size         string `json:"size"`
	Type         string `json:"type"`
	LastModified string `json:"last_modified"`
}

type getFileListData struct {
	Location string `json:"location"`
}

// GetFileList 获取文件列表
func GetFileList(c *gin.Context) {
	var data getFileListData
	if err := c.ShouldBindJSON(&data); err != nil {
		apiException.AbortWithException(c, apiException.ParamError, err)
		return
	}

	path := filepath.Join(config.OSSFolder, objectService.CleanLocation(data.Location))
	stat, err := os.Stat(path)
	if os.IsNotExist(err) || !stat.IsDir() {
		apiException.AbortWithException(c, apiException.LocationNotFound, err)
		return
	}

	fileList, err := os.ReadDir(path)
	if err != nil {
		apiException.AbortWithException(c, apiException.ServerError, err)
		return
	}

	list := make([]fileListElement, 0)
	for _, file := range fileList {
		fileInfo, err := file.Info()
		if err != nil {
			zap.L().Error("获取文件信息错误", zap.Error(err))
			continue
		}

		fullPath := filepath.Join(path, fileInfo.Name())
		sizeKB := float64(fileInfo.Size()) / 1024
		list = append(list, fileListElement{
			Name:         fileInfo.Name(),
			Size:         fmt.Sprintf("%.2f", sizeKB), // 保留两位小数
			Type:         objectService.GetFileType(fullPath, fileInfo.IsDir()),
			LastModified: fileInfo.ModTime().Format(time.RFC3339),
		})
	}

	response.JsonSuccessResp(c, gin.H{
		"file_list": list,
	})
}
