package server

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"
)

// Run 运行 Http 服务器
func Run(handler http.Handler, addr string) {
	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 2 * time.Second,
	}

	// 启动服务器协程
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			zap.L().Fatal("Server error occurred", zap.Error(err))
		}
	}()

	// 阻塞并监听结束信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	zap.L().Info("Shutdown server...")

	// 关闭服务器（5秒超时时间）
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		zap.L().Error("Server shutdown failed", zap.Error(err))
	}

	zap.L().Info("Server closed")
}
