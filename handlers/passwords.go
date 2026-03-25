package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type AudioMemo struct {
	Data      string `json:"data"`
	MimeType  string `json:"mimeType"`
	Duration  int    `json:"duration"`
	CreatedAt int64  `json:"createdAt"`
}

type Password struct {
	ID        int        `json:"id"`
	Title     string     `json:"title"`
	Username  string     `json:"username"`
	Password  string     `json:"password"`
	URL       string     `json:"url"`
	Notes     string     `json:"notes"`
	Labels    []string   `json:"labels"`
	AudioMemo *AudioMemo `json:"audioMemo,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

const passwordsFile = "passwords.json"

var (
	passwords   = make(map[int]Password)
	pwNextID    = 1
	passwordsMu sync.Mutex
)

func LoadPasswords() {
	if _, err := os.Stat(passwordsFile); os.IsNotExist(err) {
		return
	}
	data, err := os.ReadFile(passwordsFile)
	if err != nil {
		log.Println("Failed to read passwords file:", err)
		return
	}
	var ps []Password
	if err := json.Unmarshal(data, &ps); err != nil {
		log.Println("Failed to parse passwords file:", err)
		return
	}
	for _, p := range ps {
		passwords[p.ID] = p
		if p.ID >= pwNextID {
			pwNextID = p.ID + 1
		}
	}
}

func savePasswords() {
	ps := make([]Password, 0, len(passwords))
	for _, p := range passwords {
		ps = append(ps, p)
	}
	data, _ := json.MarshalIndent(ps, "", "  ")
	_ = os.WriteFile(passwordsFile, data, 0644)
}

func GetPasswords(c *gin.Context) {
	passwordsMu.Lock()
	defer passwordsMu.Unlock()
	ps := make([]Password, 0, len(passwords))
	for _, p := range passwords {
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

	passwordsMu.Lock()
	pw.ID = pwNextID
	pwNextID++
	now := time.Now()
	pw.CreatedAt = now
	pw.UpdatedAt = now
	if pw.Labels == nil {
		pw.Labels = []string{}
	}
	passwords[pw.ID] = pw
	savePasswords()
	passwordsMu.Unlock()

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

	passwordsMu.Lock()
	existing, ok := passwords[id]
	if !ok {
		passwordsMu.Unlock()
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
	passwords[id] = existing
	savePasswords()
	passwordsMu.Unlock()

	c.JSON(http.StatusOK, existing)
}

func DeletePassword(c *gin.Context) {
	idStr := c.Param("id")
	id, _ := strconv.Atoi(idStr)

	passwordsMu.Lock()
	_, ok := passwords[id]
	if ok {
		delete(passwords, id)
		savePasswords()
	}
	passwordsMu.Unlock()

	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Password not found"})
		return
	}

	c.Status(http.StatusNoContent)
}
