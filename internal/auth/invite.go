// Package auth parses the invite-code registry used for multi-user login.
//
// Invite codes are configured by the admin via the INVITE_CODES environment
// variable, e.g. INVITE_CODES="alice=code-a1b2,bob=code-c3d4". Each code maps
// to a username; the code itself is the login credential.
package auth

import (
	"crypto/subtle"
	"fmt"
	"regexp"
	"strings"
)

// usernameRule 限制用户名字符集：用户名会用于拼数据文件名（data/notes-<user>.json），
// 只允许 [a-zA-Z0-9_-]，杜绝路径穿越。
var usernameRule = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// AdminUsername 是 AUTH_PASSWORD 兜底通道对应的用户名。
const AdminUsername = "admin"

// Registry 是邀请码 -> 用户名的注册表，启动时从环境变量解析，运行期只读。
type Registry struct {
	byCode    map[string]string // inviteCode -> username
	usernames []string          // 按 INVITE_CODES 中出现顺序，用于旧数据迁移归属
}

// ParseInviteCodes 解析 INVITE_CODES 环境变量。
// 空字符串返回空 Registry（合法，表示纯 AUTH_PASSWORD legacy 模式）；
// 非法条目（缺 '='、空 username/code、重复、用户名含非法字符）返回 error，调用方应启动失败。
func ParseInviteCodes(env string) (*Registry, error) {
	r := &Registry{byCode: make(map[string]string)}
	env = strings.TrimSpace(env)
	if env == "" {
		return r, nil
	}

	seenUser := make(map[string]bool)
	for _, entry := range strings.Split(env, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid INVITE_CODES entry %q: want username=code", entry)
		}
		username := strings.TrimSpace(parts[0])
		code := strings.TrimSpace(parts[1])
		if username == "" || code == "" {
			return nil, fmt.Errorf("invalid INVITE_CODES entry %q: empty username or code", entry)
		}
		if !usernameRule.MatchString(username) {
			return nil, fmt.Errorf("invalid username %q: only [a-zA-Z0-9_-] allowed", username)
		}
		if username == AdminUsername {
			return nil, fmt.Errorf("username %q is reserved for the AUTH_PASSWORD channel", AdminUsername)
		}
		if seenUser[username] {
			return nil, fmt.Errorf("duplicate username %q in INVITE_CODES", username)
		}
		if _, dup := r.byCode[code]; dup {
			return nil, fmt.Errorf("duplicate invite code in INVITE_CODES (user %q)", username)
		}
		seenUser[username] = true
		r.byCode[code] = username
		r.usernames = append(r.usernames, username)
	}
	return r, nil
}

// Empty 表示未配置任何邀请码。
func (r *Registry) Empty() bool {
	return len(r.byCode) == 0
}

// Lookup 用邀请码查用户名。遍历 + 常数时间比对，避免时序侧信道。
func (r *Registry) Lookup(code string) (string, bool) {
	for c, u := range r.byCode {
		if subtle.ConstantTimeCompare([]byte(c), []byte(code)) == 1 {
			return u, true
		}
	}
	return "", false
}

// IsActive 报告用户名当前是否在册（含 admin 通道，admin 恒在册）。
func (r *Registry) IsActive(username string) bool {
	if username == AdminUsername {
		return true
	}
	for _, u := range r.usernames {
		if u == username {
			return true
		}
	}
	return false
}

// FirstUsername 返回配置顺序中的第一个用户名（用于旧数据迁移归属），无用户时返回 ""。
func (r *Registry) FirstUsername() string {
	if len(r.usernames) == 0 {
		return ""
	}
	return r.usernames[0]
}

// Usernames 返回全部在册用户名（不含 admin），供启动时预加载数据。
func (r *Registry) Usernames() []string {
	return append([]string(nil), r.usernames...)
}

// WarnWeakCodes 返回强度不足的邀请码（<8 字符），供启动时打警告日志，不拦截。
func (r *Registry) WarnWeakCodes() []string {
	var weak []string
	for code, u := range r.byCode {
		if len(code) < 8 {
			weak = append(weak, u)
		}
	}
	return weak
}
