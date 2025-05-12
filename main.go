package main

import (
	"fmt"
	"github.com/chai2010/webp"
	"github.com/gin-gonic/gin"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"file-converter/handlers"
)

func main() {
	r := gin.Default()

	r.Static("/static", "./static")
	r.LoadHTMLGlob("templates/*")

	r.POST("/concat", handlers.UploadHandler)
	r.POST("/convert", ConvertHandler)
	r.GET("/download/:filename", func(c *gin.Context) {
		filename := c.Param("filename")
		filePath := filepath.Join("./tmp", filename)
		c.FileAttachment(filePath, filename)
	})
	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", nil)
	})
	r.GET("/file-convert", func(c *gin.Context) {
		c.HTML(http.StatusOK, "file-convert.html", nil)
	})
	r.GET("/png-to-pdf", func(c *gin.Context) {
		c.HTML(http.StatusOK, "png2pdf.html", nil)
	})

	os.MkdirAll("./tmp", os.ModePerm)
	go AutoCleanTmp()

	r.Run(":8080")
}

func ConvertHandler(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.String(http.StatusBadRequest, "No file uploaded")
		return
	}

	// 限制文件大小为 5MB（单位：字节）
	const maxFileSize = 5 * 1024 * 1024 // 5MB
	if file.Size > maxFileSize {
		c.String(http.StatusBadRequest, "File size cannot exceed 5MB")
		return
	}

	target := c.PostForm("target")

	uploadPath := filepath.Join("./tmp", file.Filename)
	err = c.SaveUploadedFile(file, uploadPath)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to save file")
		return
	}

	convertedName := fmt.Sprintf("%d.%s", time.Now().UnixNano(), target)
	convertedPath := filepath.Join("./tmp", convertedName)

	// if strings.ToLower(target) == "webp" {
	// 	err = ConvertToWebP(uploadPath, convertedPath)
	// 	if err != nil {
	// 		c.String(http.StatusInternalServerError, "Conversion failed: %s", err.Error())
	// 		return
	// 	}
	// } else {
	// 	c.String(http.StatusBadRequest, "Unsupported target format")
	// 	return
	// }

	switch strings.ToLower(target) {
	case "webp":
		err = ConvertToWebP(uploadPath, convertedPath)
	case "docx":
		// PDF to Word (.docx)
		convertedPath, err = handlers.ConvertToDocx(uploadPath)
	case "pdf":
		// Word (.doc/.docx) to PDF
		convertedPath, err = handlers.ConvertToPDF(uploadPath)
	default:
		c.String(http.StatusBadRequest, "Unsupported target format")
		return
	}

	// Extract filename from converted path
	_, fileName := filepath.Split(convertedPath)
	if fileName == "" {
		c.JSON(http.StatusOK, gin.H{
			"msg": "convert failed!",
		})

		return
	}

	c.JSON(http.StatusOK, gin.H{
		"download_url": "/download/" + fileName,
	})
}

func ConvertToWebP(inputPath, outputPath string) error {
	f, err := os.Open(inputPath)
	if err != nil {
		return err
	}
	defer f.Close()

	img, format, err := image.Decode(f)
	if err != nil {
		return fmt.Errorf("failed to decode image: %v", err)
	}
	if format != "jpeg" && format != "png" {
		return fmt.Errorf("unsupported image format: %s", format)
	}

	out, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer out.Close()

	return webp.Encode(out, img, &webp.Options{Lossless: true})
}

func AutoCleanTmp() {
	ticker := time.NewTicker(5 * time.Minute)
	for range ticker.C {
		files, err := os.ReadDir("./tmp")
		if err != nil {
			continue
		}
		now := time.Now()
		for _, f := range files {
			path := filepath.Join("./tmp", f.Name())
			info, err := os.Stat(path)
			if err == nil && now.Sub(info.ModTime()) > 10*time.Minute {
				os.Remove(path)
			}
		}
	}
}
