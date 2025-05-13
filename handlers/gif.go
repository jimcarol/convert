package handlers

import (
	"fmt"
	"image"
	"image/color/palette"
	"image/draw"
	"image/gif"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/fogleman/gg"
	"github.com/gin-gonic/gin"
)

func convertToPaletted(img image.Image) *image.Paletted {
	bounds := img.Bounds()
	// 使用 nil 会自动生成调色板
	paletted := image.NewPaletted(bounds, palette.Plan9)
	draw.FloydSteinberg.Draw(paletted, bounds, img, image.Point{})
	return paletted
}

func UploadGIFHandler(c *gin.Context) {
	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid form data"})
		return
	}

	files := form.File["images"]
	texts := form.Value["texts"] // 可选文本

	if len(files) == 0 {
		c.JSON(400, gin.H{"error": "No images uploaded"})
		return
	}

	var frames []*image.Paletted
	var delays []int

	for i, file := range files {
		src, err := file.Open()
		if err != nil {
			c.JSON(500, gin.H{"error": "Failed to open image"})
			return
		}
		img, _, err := image.Decode(src)
		src.Close()
		if err != nil {
			c.JSON(500, gin.H{"error": "Decode image failed"})
			return
		}

		dc := gg.NewContextForImage(img)

		if i < len(texts) && texts[i] != "" {
			// dc.SetRGB(1, 1, 1) // 白色字体
			dc.SetRGB(1, 0, 0) // 红色
			fontPath := os.Getenv("FONT_PATH")
			if fontPath == "" {
				fontPath = "/System/Library/Fonts/Palatino.ttc"
			}
			// fontPath := "/System/Library/Fonts/Palatino.ttc"
			if err := dc.LoadFontFace(fontPath, 24); err != nil {
				c.JSON(500, gin.H{"error": "Font loading failed"})
				return
			}
			dc.DrawStringAnchored(texts[i], float64(dc.Width()/2), float64(dc.Height()-40), 0.5, 0.5)
		}

		paletted := convertToPaletted(dc.Image())
		frames = append(frames, paletted)
		delays = append(delays, 100) // 每帧延迟 100 ticks (单位 1/100 秒)
	}

	
	outputPath := filepath.Join("tmp", fmt.Sprintf("output_%d.gif", time.Now().UnixNano()))
	outFile, err := os.Create(outputPath)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to create output file"})
		return
	}
	defer outFile.Close()

	anim := gif.GIF{
		Image: frames,
		Delay: delays,
	}

	if err := gif.EncodeAll(outFile, &anim); err != nil {
		log.Println("Error occurred:", err)
		c.JSON(500, gin.H{"error": "Failed to encode gif"})
		return
	}

  _, fileName := filepath.Split(outputPath)
	c.JSON(200, gin.H{
		"message": "GIF created",
		"url":     "/download/" + fileName,
	})
}
