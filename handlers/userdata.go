package handlers

import (
	"log"
	"os"
	"path/filepath"
)

// 多用户数据按用户名分文件存放：data/notes-<user>.json、data/passwords-<user>.json。
// 单文件 schema 与旧版完全一致（顶层 JSON 数组，无 owner 字段），隔离靠文件边界保证。
var dataDir = "data"

// InitUserStores 设置数据目录并完成旧数据迁移，启动时调用一次。
// defaultOwner：旧单文件（notes.json/passwords.json）的归属用户，
// 传 INVITE_CODES 里第一个用户名；纯 legacy 模式传 admin。
func InitUserStores(dir, defaultOwner string) {
	if dir != "" {
		dataDir = dir
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		log.Printf("failed to create data dir %s: %v", dataDir, err)
		return
	}
	migrateLegacyData(defaultOwner)
}

func userDataFile(kind, username string) string {
	return filepath.Join(dataDir, kind+"-"+username+".json")
}

// migrateLegacyData 把 repo 根目录的旧单文件 rename 到 defaultOwner 的 per-user 文件。
// 目标文件已存在时不覆盖（保留两份，打日志提醒人工处理）。
func migrateLegacyData(defaultOwner string) {
	if defaultOwner == "" {
		return
	}
	for _, kind := range []string{"notes", "passwords"} {
		legacy := kind + ".json"
		if _, err := os.Stat(legacy); err != nil {
			continue
		}
		target := userDataFile(kind, defaultOwner)
		if _, err := os.Stat(target); err == nil {
			log.Printf("legacy %s exists but %s already present; leaving %s untouched", legacy, target, legacy)
			continue
		}
		if err := os.Rename(legacy, target); err != nil {
			log.Printf("failed to migrate %s -> %s: %v", legacy, target, err)
			continue
		}
		log.Printf("migrated %s -> %s", legacy, target)
	}
}
