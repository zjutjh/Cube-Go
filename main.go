package main

import (
	"os"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"jh-oss/internal/midwares"
	"jh-oss/internal/routes"
	"jh-oss/pkg/config"
	"jh-oss/pkg/log"
	"jh-oss/pkg/server"
)

func main() {
	if !config.Config.GetBool("server.debug") {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.Default()
	r.Use(cors.Default())
	r.Use(midwares.ErrHandler())
	r.NoMethod(midwares.HandleNotFound)
	r.NoRoute(midwares.HandleNotFound)
	log.Init()
	routes.Init(r)

	// 确保存储文件夹存在，如果不存在则创建
	if _, err := os.Stat(config.OSSFolder); os.IsNotExist(err) {
		err := os.Mkdir(config.OSSFolder, os.ModePerm)
		if err != nil {
			zap.L().Fatal("Failed to create static directory", zap.Error(err))
		}
	}

	// 设置静态文件服务
	// r.StaticFS("/"+config.OSSFolder, http.Dir(config.OSSFolder))

	server.Run(r, ":"+config.Config.GetString("server.port"))
}
