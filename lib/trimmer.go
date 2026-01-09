package lib

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

type TrimmerConfig struct {
	InputFile  string
	OutputName string
	OutputDir  string
	Type       string
	Categories []string
}

type Trimmer struct {
	config TrimmerConfig
}

func NewTrimmer(config TrimmerConfig) *Trimmer {
	return &Trimmer{config: config}
}

func (t *Trimmer) Trim() error {
	if !fileExists(t.config.InputFile) {
		return fmt.Errorf("input file not found: %s", t.config.InputFile)
	}

	fileSize, _ := getFileSize(t.config.InputFile)
	fmt.Printf("[%s] Trimming %s... (%.2f MB)\n", stringToUpper(t.config.Type), filepath.Base(t.config.InputFile), fileSize)

	data, err := os.ReadFile(t.config.InputFile)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	if err := os.MkdirAll(t.config.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	filterMap := make(map[string]bool)
	for _, cat := range t.config.Categories {
		filterMap[cat] = true
	}

	trimmedData, keptCount, err := t.trimData(data, filterMap, t.config.Type)
	if err != nil {
		return err
	}

	outputPath := filepath.Join(t.config.OutputDir, t.config.OutputName)
	if err := os.WriteFile(outputPath, trimmedData, 0644); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	originalStr := formatSize(int64(len(data)))
	trimmedStr := formatSize(int64(len(trimmedData)))

	fmt.Printf("Original: %s, Trimmed: %s\n", originalStr, trimmedStr)
	fmt.Printf("✓ Kept %d categories\n", keptCount)
	fmt.Printf("✓ Created %s\n", filepath.Base(outputPath))
	return nil
}

func (t *Trimmer) trimData(data []byte, filterMap map[string]bool, typeStr string) ([]byte, int, error) {
	var entries [][]byte
	var codes []string

	reader := bytes.NewReader(data)
	for reader.Len() > 0 {
		tag, err := readVarint(reader)
		if err != nil {
			break
		}

		fieldNumber := tag >> 3
		wireType := tag & 0x07

		if fieldNumber == 1 && wireType == 2 {
			// Read entry
			length, err := readVarint(reader)
			if err != nil {
				break
			}

			msgData := make([]byte, length)
			n, err := reader.Read(msgData)
			if err != nil || n != int(length) {
				break
			}

			code := parseEntryForCountryCode(msgData)

			if len(filterMap) == 0 || filterMap[code] {
				entries = append(entries, msgData)
				codes = append(codes, code)
			}
		} else {
			if wireType == 0 {
				readVarint(reader)
			} else if wireType == 2 {
				length, _ := readVarint(reader)
				reader.Read(make([]byte, length))
			} else if wireType == 5 {
				reader.Read(make([]byte, 4))
			} else if wireType == 1 {
				reader.Read(make([]byte, 8))
			}
		}
	}

	if len(filterMap) == 0 {
		entries = nil
		for _, code := range codes {
			entry := marshalEmptyEntry(code)
			entries = append(entries, entry)
		}
	}

	var testCatName string
	if typeStr == "geoip" {
		testCatName = "TESTIP"
	} else if typeStr == "geosite" {
		testCatName = "TESTSITE"
	}

	testEntry := marshalEmptyEntry(testCatName)
	entries = append(entries, testEntry)
	codes = append(codes, testCatName)

	type codeEntry struct {
		code string
		data []byte
	}

	var sorted []codeEntry
	for i, code := range codes {
		sorted = append(sorted, codeEntry{code: code, data: entries[i]})
	}

	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].code < sorted[j].code
	})

	var buf bytes.Buffer
	for _, item := range sorted {
		writeFieldTo(&buf, 1, 2, item.data)
	}

	return buf.Bytes(), len(sorted), nil
}

func marshalEmptyEntry(countryCode string) []byte {
	var buf bytes.Buffer
	writeString(&buf, 1, countryCode)
	return buf.Bytes()
}

func stringToUpper(s string) string {
	result := ""
	for _, r := range s {
		if r >= 'a' && r <= 'z' {
			result += string(rune(r - 32))
		} else {
			result += string(r)
		}
	}
	return result
}

func formatSize(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	} else if bytes < 1024*1024 {
		return fmt.Sprintf("%.2f KB", float64(bytes)/1024)
	} else {
		return fmt.Sprintf("%.2f MB", float64(bytes)/1024/1024)
	}
}
