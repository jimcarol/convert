package pdfgen

import (
	"github.com/signintech/gopdf"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"math"
)

const (
	pageWidth  = 595.28 // A4 纸宽度 pt
	pageHeight = 841.89 // A4 纸高度 pt
	margin     = 20.0   // 边距 pt
)

// AddCenteredImage 添加等比缩放并居中的图片到 PDF 页面
func AddCenteredImage(pdf *gopdf.GoPdf, img image.Image, imgPath string) error {
	imgW := float64(img.Bounds().Dx())
	imgH := float64(img.Bounds().Dy())

	// 可用区域尺寸
	maxW := pageWidth - 2*margin
	maxH := pageHeight - 2*margin

	// 缩放比例
	scale := math.Min(maxW/imgW, maxH/imgH)

	finalW := imgW * scale
	finalH := imgH * scale

	// 居中位置
	x := (pageWidth - finalW) / 2
	y := (pageHeight - finalH) / 2

	pdf.AddPage()
	return pdf.Image(imgPath, x, y, &gopdf.Rect{W: finalW, H: finalH})
}


// GeneratePDF 接收多个图片路径，生成 PDF 并保存为目标文件
func GeneratePDF(imagePaths []string, outputPath string) error {
	pdf := gopdf.GoPdf{}
	pdf.Start(gopdf.Config{PageSize: *gopdf.PageSizeA4}) // 默认 A4 页面

	for _, path := range imagePaths {
		// 打开图片
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		img, _, err := image.Decode(f)
		f.Close()
		if err != nil {
			return err
		}

		err = AddCenteredImage(&pdf, img, path)
		if err != nil {
			return err
		}
	}

	// 保存 PDF
	return pdf.WritePdf(outputPath)
}
