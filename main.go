package main

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"jh-oss/internal/midwares"
	"jh-oss/internal/routes"
	"jh-oss/pkg/config"
	"jh-oss/pkg/log"
	"jh-oss/pkg/oss"
	"jh-oss/pkg/server"
)

func main() {
	if !config.Config.GetBool("server.debug") {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.Default()
	r.Use(server.InitCORS())
	r.Use(midwares.ErrHandler())
	r.NoMethod(midwares.HandleNotFound)
	r.NoRoute(midwares.HandleNotFound)
	log.Init()
	if err := oss.Init(); err != nil {
		zap.L().Fatal("Init OSS failed", zap.Error(err))
	}
	routes.Init(r)
	server.Run(r, ":"+config.Config.GetString("server.port"))
}
