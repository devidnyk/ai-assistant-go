package common

import (
	"ai-assistant/configs"
	"ai-assistant/internal/models"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/ledongthuc/pdf"
)

// readLocalFile reads a file from local filesystem
func readLocalFile(filePath string) (string, *models.Payload, error) {
	ext := strings.ToLower(filepath.Ext(filePath))
	payload := models.Payload{
		Context:    "This is a user file and will be used to answer queries.",
		DataSource: filePath,
		SourceType: configs.Local,
		Command:    "",
		Metadata:   nil,
	}

	switch ext {
	case ".txt", ".md":
		data, err := os.ReadFile(filePath)
		if err != nil {
			return "", nil, err
		}
		return string(data), &payload, nil

	case ".pdf":
		data, err := readPDFContent(filePath)
		if err != nil {
			return "", nil, err
		}
		return data, &payload, nil

	default:
		return "", nil, fmt.Errorf("unsupported file type: %s (supported: .txt, .md, .pdf)", ext)
	}
}

// readRemoteFile reads a file from a URL
func readRemoteFile(url string) (string, *models.Payload, error) {
	resp, err := http.Get(url)
	payload := models.Payload{
		Context:    "This is a user file and will be used to answer queries.",
		DataSource: url,
		SourceType: configs.GDrive,
		Command:    "",
		Metadata:   nil,
	}

	if err != nil {
		return "", nil, fmt.Errorf("failed to fetch remote file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("failed to fetch remote file: status %d", resp.StatusCode)
	}

	// Determine file type from URL or Content-Type
	ext := strings.ToLower(filepath.Ext(url))
	contentType := resp.Header.Get("Content-Type")

	// For PDF files
	if ext == ".pdf" || strings.Contains(contentType, "pdf") {
		// Download to temp file first, then read
		tempFile, err := os.CreateTemp("", "remote-*.pdf")
		if err != nil {
			return "", nil, fmt.Errorf("failed to create temp file: %w", err)
		}
		defer os.Remove(tempFile.Name())
		defer tempFile.Close()

		if _, err := io.Copy(tempFile, resp.Body); err != nil {
			return "", nil, fmt.Errorf("failed to download PDF: %w", err)
		}

		data, err := readPDFContent(tempFile.Name())
		if err != nil {
			return "", nil, fmt.Errorf("failed to read PDF content: %w", err)
		}

		return data, nil, nil
	}

	// For text/markdown files
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, fmt.Errorf("failed to read remote file: %w", err)
	}

	return string(data), &payload, nil
}

// readPDFContent extracts text content from a PDF file
func readPDFContent(filePath string) (string, error) {
	f, r, err := pdf.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open PDF: %w", err)
	}
	defer f.Close()

	var content strings.Builder
	totalPages := r.NumPage()

	for pageIndex := 1; pageIndex <= totalPages; pageIndex++ {
		p := r.Page(pageIndex)
		if p.V.IsNull() {
			continue
		}

		text, err := p.GetPlainText(nil)
		if err != nil {
			return "", fmt.Errorf("failed to extract text from page %d: %w", pageIndex, err)
		}
		content.WriteString(text)
		content.WriteString("\n")
	}

	return content.String(), nil
}

// ReadFile reads a file from local or remote source and returns content as text
// Supports .txt, .md, and .pdf formats
func ReadFile(sourceLocType configs.SourceDataType, sourceDataUrl string) (string, *models.Payload, error) {
	switch sourceLocType {
	case configs.Local:
		return readLocalFile(sourceDataUrl)
	case configs.GDrive:
		return readRemoteFile(sourceDataUrl)
	default:
		return "", nil, fmt.Errorf("unsupported source type: %v", sourceLocType)
	}
}
