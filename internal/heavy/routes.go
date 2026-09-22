package heavy

import (
	"file-converter/handlers"
	"file-converter/middleware"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chai2010/webp"
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.Engine, jwtSecret string) {
	api := r.Group("/")
	api.Use(middleware.AuthRequired(jwtSecret))
	RegisterProtectedRoutes(api)
}

// RegisterProtectedRoutes 注册 heavy 类转换 API（pdfgen/word-pdf/gif）。
// 仅限 admin：资源消耗大，普通登录用户返回 403。
// 调用方需已在 api 上挂 AuthRequired。
func RegisterProtectedRoutes(api *gin.RouterGroup) {
	admin := api.Group("/")
	admin.Use(middleware.AdminRequired())
	admin.POST("/concat", handlers.UploadHandler)
	admin.POST("/convert", ConvertHandler)
	admin.POST("/upload-gif", handlers.UploadGIFHandler)
	admin.GET("/download/:filename", func(c *gin.Context) {
		filename := filepath.Base(c.Param("filename"))
		filePath := filepath.Join("./tmp", filename)
		c.FileAttachment(filePath, filename)
	})
}

func ConvertHandler(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.String(http.StatusBadRequest, "No file uploaded")
		return
	}

	const maxFileSize = 5 * 1024 * 1024
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

	switch strings.ToLower(target) {
	case "webp":
		err = ConvertToWebP(uploadPath, convertedPath)
	case "docx":
		convertedPath, err = handlers.ConvertToDocx(uploadPath)
	case "pdf":
		convertedPath, err = handlers.ConvertToPDF(uploadPath)
	default:
		c.String(http.StatusBadRequest, "Unsupported target format")
		return
	}

	if err != nil {
		c.String(http.StatusInternalServerError, "Conversion failed: %s", err.Error())
		return
	}

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
				_ = os.Remove(path)
			}
		}
	}
}
