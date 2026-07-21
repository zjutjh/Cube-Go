package main

import (
	"context"
	"strings"

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
	if strings.TrimSpace(config.Config.GetString("oss.adminKey")) == "" {
		zap.L().Fatal("oss.adminKey must not be empty")
	}
	if err := oss.Init(context.Background()); err != nil {
		zap.L().Fatal("Init OSS failed", zap.Error(err))
	}
	defer func() {
		if err := oss.Close(); err != nil {
			zap.L().Error("Close OSS failed", zap.Error(err))
		}
	}()
	routes.Init(r)
	server.Run(r, ":"+config.Config.GetString("server.port"))
}
