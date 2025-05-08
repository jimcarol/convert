package handlers

import (
	"fmt"
	"os/exec"
	"path/filepath"
)

// ConvertToPDF converts Word (.docx) to PDF
func ConvertToPDF(inputPath string) (string, error) {
	return convert(inputPath, "pdf")
}

// ConvertToDocx converts PDF to Word (.docx)
func ConvertToDocx(inputPath string) (string, error) {
	return convert(inputPath, "docx")
}

func convertWithSOffice(inputPath, outputDir, targetFormat string) ([]byte, error) {
	args := []string{"--headless"}

	// 添加 infilter 参数（如果目标格式不是 PDF）
	if targetFormat != "pdf" {
		args = append(args, "--infilter=writer_pdf_import")
	}

	args = append(args,
		"--convert-to", targetFormat,
		"--outdir", outputDir,
		inputPath,
	)

	cmd := exec.Command("soffice", args...)
	return cmd.CombinedOutput()
}

func convert(inputPath, targetFormat string) (string, error) {
	outputDir := filepath.Dir(inputPath)
	out, err := convertWithSOffice(inputPath, outputDir, targetFormat)
	if err != nil {
		return "", fmt.Errorf("conversion error: %v | output: %s", err, string(out))
	}
	fmt.Println("Conversion successful!")
	fmt.Println(string(out))

	outputFile := filepath.Join(outputDir, replaceExt(filepath.Base(inputPath), "."+targetFormat))
	return outputFile, nil
}

func replaceExt(filename, newExt string) string {
	return filename[:len(filename)-len(filepath.Ext(filename))] + newExt
}
