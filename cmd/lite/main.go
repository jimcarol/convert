package main

import (
	"file-converter/handlers"
	"file-converter/internal/auth"
	"file-converter/internal/server"
	"file-converter/middleware"
	"log"
	"os"

	"github.com/gin-gonic/gin"
)

func main() {
	jwtSecret, inviteReg, authPassword := initAuth()

	r := server.NewRouter()
	server.RegisterPublicWebRoutes(r)
	server.RegisterProtectedWebRoutes(r, inviteReg, authPassword != "", jwtSecret)
	server.RegisterAuthRoutes(r, inviteReg, authPassword, jwtSecret)
	registerProtectedRoutes(r, inviteReg, authPassword != "", jwtSecret)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	if err := r.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}

// initAuth 读取并校验鉴权相关的环境变量，并初始化 per-user 数据存储。
// 返回 JWT 密钥、邀请码注册表和 admin 兜底密码。
func initAuth() (jwtSecret string, inviteReg *auth.Registry, authPassword string) {
	jwtSecret = os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("JWT_SECRET environment variable must be set")
	}

	// 多用户邀请码（INVITE_CODES="alice=code-a1b2,bob=code-c3d4"）；
	// AUTH_PASSWORD 保留为 admin 兜底通道。两者至少配置一个。
	inviteReg, err := auth.ParseInviteCodes(os.Getenv("INVITE_CODES"))
	if err != nil {
		log.Fatal("invalid INVITE_CODES: ", err)
	}
	authPassword = os.Getenv("AUTH_PASSWORD")
	if inviteReg.Empty() && authPassword == "" {
		log.Fatal("set INVITE_CODES and/or AUTH_PASSWORD")
	}
	for _, u := range inviteReg.WarnWeakCodes() {
		log.Printf("warning: invite code for user %q is shorter than 8 characters", u)
	}

	// 旧数据迁移 + per-user 分文件存储（data/notes-<user>.json 等）。
	// 旧单文件归给第一个受邀用户；纯 legacy 模式归 admin。
	defaultOwner := inviteReg.FirstUsername()
	if defaultOwner == "" {
		defaultOwner = auth.AdminUsername
	}
	handlers.InitUserStores(os.Getenv("DATA_DIR"), defaultOwner)

	return jwtSecret, inviteReg, authPassword
}

func registerProtectedRoutes(r *gin.Engine, reg *auth.Registry, allowAdmin bool, jwtSecret string) {
	api := r.Group("/")
	api.Use(middleware.AuthRequired(jwtSecret))
	api.Use(middleware.RequireActiveUser(reg, allowAdmin))
	{
		api.GET("/api/me", handlers.MeHandler)

		api.GET("/notes", handlers.GetNotes)
		api.POST("/notes", handlers.CreateNote)
		api.PUT("/notes/:id", handlers.UpdateNote)
		api.DELETE("/notes/:id", handlers.DeleteNote)

		api.GET("/passwords", handlers.GetPasswords)
		api.POST("/passwords", handlers.CreatePassword)
		api.PUT("/passwords/:id", handlers.UpdatePassword)
		api.DELETE("/passwords/:id", handlers.DeletePassword)
	}
}
