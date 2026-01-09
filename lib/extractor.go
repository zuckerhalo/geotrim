package lib

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type ExtractorConfig struct {
	InputFile  string
	OutputDir  string
	Type       string
	AddTestCat bool
}

type Extractor struct {
	config ExtractorConfig
}

func NewExtractor(config ExtractorConfig) *Extractor {
	return &Extractor{config: config}
}

func (e *Extractor) Extract() ([]string, error) {
	if !fileExists(e.config.InputFile) {
		return nil, fmt.Errorf("input file not found: %s", e.config.InputFile)
	}

	fileSize, _ := getFileSize(e.config.InputFile)
	fmt.Printf("[%s] Reading %s... (%.2f MB)\n", strings.ToUpper(e.config.Type), filepath.Base(e.config.InputFile), fileSize)

	if err := os.MkdirAll(e.config.OutputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	data, err := os.ReadFile(e.config.InputFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var categories []string

	if e.config.Type == "geoip" {
		categories, err = e.extractGeoIP(data)
	} else if e.config.Type == "geosite" {
		categories, err = e.extractGeoSite(data)
	} else {
		return nil, fmt.Errorf("unknown type: %s", e.config.Type)
	}

	if err != nil {
		return nil, err
	}

	sort.Strings(categories)

	fmt.Printf("Creating %d category files...\n", len(categories))
	for _, cat := range categories {
		filePath := filepath.Join(e.config.OutputDir, cat)
		if err := os.WriteFile(filePath, []byte{}, 0644); err != nil {
			return nil, fmt.Errorf("failed to create file %s: %w", filePath, err)
		}
	}

	if e.config.AddTestCat {
		testCat := "TESTSITE"
		if e.config.Type == "geoip" {
			testCat = "TESTIP"
		}
		testPath := filepath.Join(e.config.OutputDir, testCat)
		if err := os.WriteFile(testPath, []byte{}, 0644); err != nil {
			return nil, fmt.Errorf("failed to create test category: %w", err)
		}
		categories = append(categories, testCat)
	}

	fmt.Printf("✓ Extracted %d %s categories\n", len(categories), e.config.Type)
	fmt.Printf("✓ Created files in %s\n", e.config.OutputDir)
	return categories, nil
}

func (e *Extractor) extractGeoIP(data []byte) ([]string, error) {
	return extractCategoriesFromBinary(data, "geoip")
}

func (e *Extractor) extractGeoSite(data []byte) ([]string, error) {
	return extractCategoriesFromBinary(data, "geosite")
}

func extractCategoriesFromBinary(data []byte, dataType string) ([]string, error) {
	seen := make(map[string]bool)
	var categories []string

	reader := bytes.NewReader(data)
	for reader.Len() > 0 {
		tag, err := readVarint(reader)
		if err != nil {
			break
		}

		fieldNumber := tag >> 3
		wireType := tag & 0x07

		if fieldNumber == 1 && wireType == 2 {
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
			if code != "" && !seen[code] {
				categories = append(categories, code)
				seen[code] = true
			}
		} else {
			if wireType == 0 {
				_, err = readVarint(reader)
				if err != nil {
					break
				}
			} else if wireType == 2 {
				length, err := readVarint(reader)
				if err != nil {
					break
				}
				reader.Read(make([]byte, length))
			} else if wireType == 5 {
				reader.Read(make([]byte, 4))
			} else if wireType == 1 {
				reader.Read(make([]byte, 8))
			}
		}
	}

	return categories, nil
}

func parseEntryForCountryCode(msgData []byte) string {
	reader := bytes.NewReader(msgData)

	for reader.Len() > 0 {
		tag, err := readVarint(reader)
		if err != nil {
			break
		}

		fieldNumber := tag >> 3
		wireType := tag & 0x07

		if fieldNumber == 1 && wireType == 2 {
			length, err := readVarint(reader)
			if err != nil {
				break
			}
			codeBytes := make([]byte, length)
			reader.Read(codeBytes)
			return string(codeBytes)
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

	return ""
}

func readVarint(reader *bytes.Reader) (uint64, error) {
	var result uint64
	var shift uint

	for {
		b := make([]byte, 1)
		n, err := reader.Read(b)
		if err != nil || n == 0 {
			return 0, err
		}

		result |= uint64(b[0]&0x7f) << shift
		if b[0]&0x80 == 0 {
			break
		}
		shift += 7
	}

	return result, nil
}
