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

type Note struct {
    ID      	int    		`json:"id"`
    Title   	string 		`json:"title"`
    Content 	string 		`json:"content"`
		CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}

// File to store notes
const notesFile = "notes.json"

// In-memory cache + lock
var (
    notes   = make(map[int]Note)
    nextID  = 1
    notesMu sync.Mutex
)

// Load notes from file
func LoadNotes() {
    if _, err := os.Stat(notesFile); os.IsNotExist(err) {
        return
    }
    data, err := os.ReadFile(notesFile)
    if err != nil {
        log.Println("Failed to read notes file:", err)
        return
    }
    var ns []Note
    if err := json.Unmarshal(data, &ns); err != nil {
        log.Println("Failed to parse notes file:", err)
        return
    }
    for _, n := range ns {
        notes[n.ID] = n
        if n.ID >= nextID {
            nextID = n.ID + 1
        }
    }
}

// Save notes to file
func saveNotes() {
    ns := make([]Note, 0, len(notes))
    for _, n := range notes {
        ns = append(ns, n)
    }
    data, _ := json.MarshalIndent(ns, "", "  ")
    _ = os.WriteFile(notesFile, data, 0644)
}

// Handlers

func GetNotes(c *gin.Context) {
	notesMu.Lock()
	defer notesMu.Unlock()
	ns := make([]Note, 0, len(notes))
	for _, n := range notes {
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

	notesMu.Lock()
	note.ID = nextID
	nextID++
	now := time.Now()
	note.CreatedAt = now
	note.UpdatedAt = now
	notes[note.ID] = note
	saveNotes()
	notesMu.Unlock()

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

	notesMu.Lock()
	existing, ok := notes[id]
	if !ok {
			notesMu.Unlock()
			c.JSON(http.StatusNotFound, gin.H{"error": "Note not found"})
			return
	}

	existing.Title = note.Title
	existing.Content = note.Content
	existing.UpdatedAt = time.Now()
	notes[id] = existing
	saveNotes()
	notesMu.Unlock()

	c.JSON(http.StatusOK, existing)
}

func DeleteNote(c *gin.Context) {
  idStr := c.Param("id")
	id, _ := strconv.Atoi(idStr)

	notesMu.Lock()
	_, ok := notes[id]
	if ok {
			delete(notes, id)
			saveNotes()
	}
	notesMu.Unlock()

	if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": "Note not found"})
			return
	}

	c.Status(http.StatusNoContent)
}