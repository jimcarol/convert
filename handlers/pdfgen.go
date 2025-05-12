package handlers

import (
	"fmt"
	"time"
	"github.com/gin-gonic/gin"
	"io"
	"io/ioutil"
	"mime/multipart"
	"os"
	"path/filepath"
	"file-converter/pdfgen"
)

func UploadHandler(c *gin.Context) {
	form, err := c.MultipartForm()
	if err != nil {
		c.String(400, "Invalid form data")
		return
	}

	files := form.File["images"]
	var imagePaths []string

	// 临时保存上传的图片
	for _, file := range files {
		tempFile, err := saveTempFile(file)
		if err != nil {
			c.String(500, "Failed to save image")
			return
		}
		imagePaths = append(imagePaths, tempFile)
		defer os.Remove(tempFile)
	}

	// 输出 PDF 路径
	pdfPath := filepath.Join("tmp", fmt.Sprintf("output_%d.pdf", time.Now().UnixNano()))
	err = pdfgen.GeneratePDF(imagePaths, pdfPath)
	if err != nil {
		c.String(500, "PDF 生成失败")
		return
	}
	defer os.Remove(pdfPath)

	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Disposition", "attachment; filename=images.pdf")
	c.File(pdfPath)
}

// saveTempFile 将上传的文件保存为本地临时文件
func saveTempFile(file *multipart.FileHeader) (string, error) {
	src, err := file.Open()
	if err != nil {
		return "", err
	}
	defer src.Close()

	tempFile, err := ioutil.TempFile("", "*.jpg") // 或 *.png
	if err != nil {
		return "", err
	}
	defer tempFile.Close()

	_, err = io.Copy(tempFile, src)
	return tempFile.Name(), err
}
