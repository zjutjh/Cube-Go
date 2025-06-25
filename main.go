package main

import (
	"cube-go/internal/midwares"
	"cube-go/internal/routes"
	"cube-go/pkg/config"
	"cube-go/pkg/log"
	"cube-go/pkg/oss"
	"cube-go/pkg/server"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
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
