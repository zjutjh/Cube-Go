package main

import (
	"os"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"jh-oss/internal/midwares"
	"jh-oss/internal/routes"
	"jh-oss/internal/utils/server"
	"jh-oss/pkg/config"
	"jh-oss/pkg/database"
	"jh-oss/pkg/log"
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
	database.Init()
	routes.Init(r)

	// 确保 static 目录存在，如果不存在则创建
	if _, err := os.Stat("static"); os.IsNotExist(err) {
		err := os.Mkdir("static", os.ModePerm)
		if err != nil {
			zap.L().Fatal("Failed to create static directory", zap.Error(err))
		}
	}
	r.Static("/static", "./static") // 挂载静态文件目录

	server.Run(r, ":"+config.Config.GetString("server.port"))
}
