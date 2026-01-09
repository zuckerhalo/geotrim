package lib

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type CreatorConfig struct {
	DataDir    string
	OutputName string
	OutputDir  string
	Type       string
}

type Creator struct {
	config CreatorConfig
}

func NewCreator(config CreatorConfig) *Creator {
	return &Creator{config: config}
}

func (c *Creator) Create() error {
	if !fileExists(c.config.DataDir) {
		return fmt.Errorf("data directory not found: %s", c.config.DataDir)
	}

	fmt.Printf("[%s] Reading categories from %s...\n", strings.ToUpper(c.config.Type), c.config.DataDir)

	categories, err := c.readCategories()
	if err != nil {
		return err
	}

	fmt.Printf("Found %d categories\n", len(categories))

	if err := os.MkdirAll(c.config.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	var data []byte
	if c.config.Type == "geoip" {
		data, err = c.createGeoIP(categories)
	} else if c.config.Type == "geosite" {
		data, err = c.createGeoSite(categories)
	} else {
		return fmt.Errorf("unknown type: %s", c.config.Type)
	}

	if err != nil {
		return err
	}

	outputPath := filepath.Join(c.config.OutputDir, c.config.OutputName)
	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	fileSize := float64(len(data)) / 1024 / 1024
	fmt.Printf("✓ Created %s (%.2f MB)\n", filepath.Base(outputPath), fileSize)
	return nil
}

func (c *Creator) readCategories() ([]string, error) {
	entries, err := os.ReadDir(c.config.DataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read data directory: %w", err)
	}

	var categories []string
	for _, entry := range entries {
		if !entry.IsDir() {
			categories = append(categories, entry.Name())
		}
	}

	sort.Strings(categories)
	return categories, nil
}

func (c *Creator) createGeoIP(categories []string) ([]byte, error) {
	var buf bytes.Buffer

	for _, cat := range categories {
		entry := marshalGeoIPEntry(cat)
		writeFieldTo(&buf, 1, 2, entry) // f1, t2
	}

	return buf.Bytes(), nil
}

func (c *Creator) createGeoSite(categories []string) ([]byte, error) {
	var buf bytes.Buffer

	for _, cat := range categories {
		// f1, f2
		entry := marshalGeoSiteEntry(cat)
		writeFieldTo(&buf, 1, 2, entry) // f1, t2
	}

	return buf.Bytes(), nil
}

func marshalGeoIPEntry(countryCode string) []byte {
	var buf bytes.Buffer
	writeString(&buf, 1, countryCode) // f1
	return buf.Bytes()
}

func marshalGeoSiteEntry(countryCode string) []byte {
	var buf bytes.Buffer
	writeString(&buf, 1, countryCode) // f1
	return buf.Bytes()
}

func writeString(buf *bytes.Buffer, fieldNum int, value string) {
	writeVarint(buf, uint64((fieldNum<<3)|2))
	writeVarint(buf, uint64(len(value)))
	buf.WriteString(value)
}

func writeFieldTo(buf *bytes.Buffer, fieldNum int, wireType int, data []byte) {
	writeVarint(buf, uint64((fieldNum<<3)|wireType))
	writeVarint(buf, uint64(len(data)))
	buf.Write(data)
}

func writeVarint(buf *bytes.Buffer, value uint64) {
	for value >= 0x80 {
		buf.WriteByte(byte(value&0x7f) | 0x80)
		value >>= 7
	}
	buf.WriteByte(byte(value))
}
