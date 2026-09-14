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

type Note struct {
    ID       int       `json:"id"`
    Title    string    `json:"title"`
    Content  string    `json:"content"`
    Tags     []string  `json:"tags,omitempty"`
    Urgent   bool      `json:"urgent,omitempty"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}

// 每个用户一个 store：自带锁 + map + nextID + 独立文件（data/notes-<user>.json），
// 多用户并发天然分片。store 懒加载，首次访问时从磁盘读取。
type noteStore struct {
	mu     sync.Mutex
	notes  map[int]Note
	nextID int
	file   string
	loaded bool
}

var (
	noteStores   = make(map[string]*noteStore)
	noteStoresMu sync.Mutex
)

func noteStoreFor(username string) *noteStore {
	noteStoresMu.Lock()
	defer noteStoresMu.Unlock()
	s, ok := noteStores[username]
	if !ok {
		s = &noteStore{
			notes:  make(map[int]Note),
			nextID: 1,
			file:   userDataFile("notes", username),
		}
		noteStores[username] = s
	}
	return s
}

func normalizeTags(tags []string) []string {
    if len(tags) == 0 {
        return nil
    }

    seen := make(map[string]struct{}, len(tags))
    normalized := make([]string, 0, len(tags))
    for _, tag := range tags {
        if tag == "" {
            continue
        }
        if _, ok := seen[tag]; ok {
            continue
        }
        seen[tag] = struct{}{}
        normalized = append(normalized, tag)
    }

    if len(normalized) == 0 {
        return nil
    }

    return normalized
}

// loadLocked 首次访问时从磁盘加载，调用方须持有 s.mu。
func (s *noteStore) loadLocked() {
	if s.loaded {
		return
	}
	s.loaded = true
	data, err := os.ReadFile(s.file)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Println("Failed to read notes file:", err)
		}
		return
	}
	var ns []Note
	if err := json.Unmarshal(data, &ns); err != nil {
		log.Println("Failed to parse notes file:", err)
		return
	}
	for _, n := range ns {
		n.Tags = normalizeTags(n.Tags)
		s.notes[n.ID] = n
		if n.ID >= s.nextID {
			s.nextID = n.ID + 1
		}
	}
}

// saveLocked 整文件重写，调用方须持有 s.mu。
func (s *noteStore) saveLocked() {
	ns := make([]Note, 0, len(s.notes))
	for _, n := range s.notes {
		ns = append(ns, n)
	}
	data, _ := json.MarshalIndent(ns, "", "  ")
	if err := os.WriteFile(s.file, data, 0644); err != nil {
		log.Println("Failed to save notes file:", err)
	}
}

// Handlers

func GetNotes(c *gin.Context) {
	s := noteStoreFor(middleware.Username(c))
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loadLocked()
	ns := make([]Note, 0, len(s.notes))
	for _, n := range s.notes {
			ns = append(ns, n)
	}
	c.JSON(http.StatusOK, ns)
}

func CreateNote(c *gin.Context) {
	var note Note
	if err := c.ShouldBindJSON(&note); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
	}

	s := noteStoreFor(middleware.Username(c))
	s.mu.Lock()
	s.loadLocked()
	note.ID = s.nextID
	s.nextID++
	note.Tags = normalizeTags(note.Tags)
	now := time.Now()
	note.CreatedAt = now
	note.UpdatedAt = now
	s.notes[note.ID] = note
	s.saveLocked()
	s.mu.Unlock()

	c.JSON(http.StatusCreated, note)
}

func UpdateNote(c *gin.Context) {
	idStr := c.Param("id")
	id, _ := strconv.Atoi(idStr)

	var note Note
	if err := c.ShouldBindJSON(&note); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
	}

	s := noteStoreFor(middleware.Username(c))
	s.mu.Lock()
	s.loadLocked()
	existing, ok := s.notes[id]
	if !ok {
			s.mu.Unlock()
			c.JSON(http.StatusNotFound, gin.H{"error": "Note not found"})
			return
	}

	existing.Title = note.Title
	existing.Content = note.Content
	existing.Tags = normalizeTags(note.Tags)
	existing.Urgent = note.Urgent
	existing.UpdatedAt = time.Now()
	s.notes[id] = existing
	s.saveLocked()
	s.mu.Unlock()

	c.JSON(http.StatusOK, existing)
}

func DeleteNote(c *gin.Context) {
  idStr := c.Param("id")
	id, _ := strconv.Atoi(idStr)

	s := noteStoreFor(middleware.Username(c))
	s.mu.Lock()
	s.loadLocked()
	_, ok := s.notes[id]
	if ok {
			delete(s.notes, id)
			s.saveLocked()
	}
	s.mu.Unlock()

	if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": "Note not found"})
			return
	}

	c.Status(http.StatusNoContent)
}
