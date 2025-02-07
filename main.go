package main

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
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

	server.Run(r, ":"+config.Config.GetString("server.port"))
}
