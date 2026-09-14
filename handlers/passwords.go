package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"file-converter/middleware"

	"github.com/gin-gonic/gin"
)

type AudioMemo struct {
	Data      string `json:"data"`
	MimeType  string `json:"mimeType"`
	Duration  int    `json:"duration"`
	CreatedAt time.Time  `json:"created_at"`
}

type Password struct {
	ID        int        `json:"id"`
	Title     string     `json:"title"`
	Username  string     `json:"username"`
	Password  string     `json:"password"`
	URL       string     `json:"url"`
	Notes     string     `json:"notes"`
	Labels    []string   `json:"labels"`
	AudioMemo *AudioMemo `json:"audio_memo,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// 与 noteStore 同构：每用户独立 store + 独立文件 data/passwords-<user>.json。
type passwordStore struct {
	mu        sync.Mutex
	passwords map[int]Password
	nextID    int
	file      string
	loaded    bool
}

var (
	passwordStores   = make(map[string]*passwordStore)
	passwordStoresMu sync.Mutex
)

func passwordStoreFor(username string) *passwordStore {
	passwordStoresMu.Lock()
	defer passwordStoresMu.Unlock()
	s, ok := passwordStores[username]
	if !ok {
		s = &passwordStore{
			passwords: make(map[int]Password),
			nextID:    1,
			file:      userDataFile("passwords", username),
		}
		passwordStores[username] = s
	}
	return s
}

// loadLocked 首次访问时从磁盘加载，调用方须持有 s.mu。
func (s *passwordStore) loadLocked() {
	if s.loaded {
		return
	}
	s.loaded = true
	data, err := os.ReadFile(s.file)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Println("Failed to read passwords file:", err)
		}
		return
	}
	var ps []Password
	if err := json.Unmarshal(data, &ps); err != nil {
		log.Println("Failed to parse passwords file:", err)
		return
	}
	for _, p := range ps {
		s.passwords[p.ID] = p
		if p.ID >= s.nextID {
			s.nextID = p.ID + 1
		}
	}
}

// saveLocked 整文件重写，调用方须持有 s.mu。
func (s *passwordStore) saveLocked() {
	ps := make([]Password, 0, len(s.passwords))
	for _, p := range s.passwords {
		ps = append(ps, p)
	}
	data, _ := json.MarshalIndent(ps, "", "  ")
	if err := os.WriteFile(s.file, data, 0644); err != nil {
		log.Println("Failed to save passwords file:", err)
	}
}

func GetPasswords(c *gin.Context) {
	s := passwordStoreFor(middleware.Username(c))
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loadLocked()
	ps := make([]Password, 0, len(s.passwords))
	for _, p := range s.passwords {
		ps = append(ps, p)
	}
	c.JSON(http.StatusOK, ps)
}

func CreatePassword(c *gin.Context) {
	var pw Password
	if err := c.ShouldBindJSON(&pw); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	s := passwordStoreFor(middleware.Username(c))
	s.mu.Lock()
	s.loadLocked()
	pw.ID = s.nextID
	s.nextID++
	now := time.Now()
	pw.CreatedAt = now
	pw.UpdatedAt = now
	if pw.Labels == nil {
		pw.Labels = []string{}
	}
	s.passwords[pw.ID] = pw
	s.saveLocked()
	s.mu.Unlock()

	c.JSON(http.StatusCreated, pw)
}

func UpdatePassword(c *gin.Context) {
	idStr := c.Param("id")
	id, _ := strconv.Atoi(idStr)

	var pw Password
	if err := c.ShouldBindJSON(&pw); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	s := passwordStoreFor(middleware.Username(c))
	s.mu.Lock()
	s.loadLocked()
	existing, ok := s.passwords[id]
	if !ok {
		s.mu.Unlock()
		c.JSON(http.StatusNotFound, gin.H{"error": "Password not found"})
		return
	}

	existing.Title = pw.Title
	existing.Username = pw.Username
	existing.Password = pw.Password
	existing.URL = pw.URL
	existing.Notes = pw.Notes
	if pw.Labels != nil {
		existing.Labels = pw.Labels
	}
	existing.AudioMemo = pw.AudioMemo
	existing.UpdatedAt = time.Now()
	s.passwords[id] = existing
	s.saveLocked()
	s.mu.Unlock()

	c.JSON(http.StatusOK, existing)
}

func DeletePassword(c *gin.Context) {
	idStr := c.Param("id")
	id, _ := strconv.Atoi(idStr)

	s := passwordStoreFor(middleware.Username(c))
	s.mu.Lock()
	s.loadLocked()
	_, ok := s.passwords[id]
	if ok {
		delete(s.passwords, id)
		s.saveLocked()
	}
	s.mu.Unlock()

	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Password not found"})
		return
	}

	c.Status(http.StatusNoContent)
}
