package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/eryajf/go-ldap-admin/logic"
	"github.com/robfig/cron/v3"

	"github.com/eryajf/go-ldap-admin/config"
	"github.com/eryajf/go-ldap-admin/middleware"
	"github.com/eryajf/go-ldap-admin/public/common"
	"github.com/eryajf/go-ldap-admin/routes"
	"github.com/eryajf/go-ldap-admin/service/isql"
)

func main() {

	// 加载配置文件到全局配置结构体
	config.InitConfig()

	// 初始化日志
	common.InitLogger()

	// 初始化数据库(mysql)
	common.InitMysql()

	// 初始化ldap连接
	common.InitLDAP()

	// 初始化casbin策略管理器
	common.InitCasbinEnforcer()

	// 初始化Validator数据校验
	common.InitValidate()

	// 初始化mysql数据
	common.InitData()

	// 操作日志中间件处理日志时没有将日志发送到rabbitmq或者kafka中, 而是发送到了channel中
	// 这里开启3个goroutine处理channel将日志记录到数据库
	for i := 0; i < 3; i++ {
		go isql.OperationLog.SaveOperationLogChannel(middleware.OperationLogChan)
	}

	// 注册所有路由
	r := routes.InitRoutes()

	host := "0.0.0.0"
	port := config.Conf.System.Port

	srv := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", host, port),
		Handler: r,
	}

	// Initializing the server in a goroutine so that
	// it won't block the graceful shutdown handling below
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			common.Log.Fatalf("listen: %s\n", err)
		}
	}()
	if config.Conf.DingTalk.DingTalkEnableSync {
		//启动定时任务
		c := cron.New(cron.WithSeconds())
		_, err := c.AddFunc("0 1 0 * * *", func() {
			common.Log.Info("每天0点1分0秒执行一次同步钉钉部门和用户信息到ldap")
			logic.DingTalk.SyncDingTalkDepts(nil, nil)
		})
		if err != nil {
			common.Log.Errorf("启动同步部门的定时任务失败: %v", err)
		}
		//每天凌晨1点执行一次
		_, err = c.AddFunc("0 15 0 * * *", func() {
			common.Log.Info("每天凌晨00点15分执行一次同步钉钉部门和用户信息到ldap")
			logic.DingTalk.SyncDingTalkUsers(nil, nil)
		})
		if err != nil {
			common.Log.Errorf("启动同步用户的定时任务失败: %v", err)
		}
		c.Start()
	}

	common.Log.Info(fmt.Sprintf("Server is running at %s:%d/%s", host, port, config.Conf.System.UrlPathPrefix))

	// Wait for interrupt signal to gracefully shutdown the server with
	// a timeout of 5 seconds.
	quit := make(chan os.Signal, 1)
	// kill (no param) default send syscall.SIGTERM
	// kill -2 is syscall.SIGINT
	// kill -9 is syscall.SIGKILL but can't be catch, so don't need add it
	// signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	signal.Notify(quit, os.Interrupt)
	<-quit
	common.Log.Info("Shutting down server...")

	// The context is used to inform the server it has 5 seconds to finish
	// the request it is currently handling
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		common.Log.Fatal("Server forced to shutdown:", err)
	}

	common.Log.Info("Server exiting!")

}
