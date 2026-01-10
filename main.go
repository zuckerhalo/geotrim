package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dustin/go-humanize"
)

const (
	fieldNumberEntry = 1
	wireTypeLength   = 2
	wireTypeVarint   = 0
	wireTypeFixed32  = 5
	wireTypeFixed64  = 1
)

func main() {
	input := flag.String("i", "", "input file path")
	output := flag.String("o", "", "output file name")
	outputDir := flag.String("od", "./", "output directory")
	typeStr := flag.String("t", "", "file type: geoip or geosite")

	flag.Parse()

	if *input == "" || *output == "" || *typeStr == "" {
		fmt.Fprintln(os.Stderr, "error: missing required flags")
		os.Exit(1)
	}

	fileType := strings.ToLower(*typeStr)
	if fileType != "geoip" && fileType != "geosite" {
		fmt.Fprintln(os.Stderr, "error: type must be 'geoip' or 'geosite'")
		os.Exit(1)
	}

	if err := trim(*input, *output, *outputDir, fileType); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func trim(inputFile, outputName, outputDir, fileType string) error {
	if _, err := os.Stat(inputFile); err != nil {
		return fmt.Errorf("input file not found: %w", err)
	}

	fmt.Printf("[%s] %s\n", strings.ToUpper(fileType), filepath.Base(inputFile))

	data, err := os.ReadFile(inputFile)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	trimmedData, keptCount, err := trimData(data, fileType)
	if err != nil {
		return err
	}

	outputPath := filepath.Join(outputDir, outputName)
	if err := os.WriteFile(outputPath, trimmedData, 0644); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	fmt.Printf("original: %s, trimmed: %s\n", humanize.Bytes(uint64(len(data))), humanize.Bytes(uint64(len(trimmedData))))
	fmt.Printf("kept %d categories\n", keptCount)
	fmt.Printf("created %s\n", filepath.Base(outputPath))
	return nil
}

func trimData(data []byte, fileType string) ([]byte, int, error) {
	codes, err := extractCountryCodes(data)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to extract country codes: %w", err)
	}

	testCatName := getTestCategoryName(fileType)
	codes = append(codes, testCatName)

	sort.Strings(codes)

	var buf bytes.Buffer
	for _, code := range codes {
		entry := marshalEmptyEntry(code)
		writeField(&buf, fieldNumberEntry, wireTypeLength, entry)
	}

	return buf.Bytes(), len(codes), nil
}

func extractCountryCodes(data []byte) ([]string, error) {
	var codes []string
	seen := make(map[string]bool)

	reader := bytes.NewReader(data)
	for reader.Len() > 0 {
		tag, err := readVarint(reader)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}

		fieldNumber := tag >> 3
		wireType := tag & 0x07

		if fieldNumber == fieldNumberEntry && wireType == wireTypeLength {
			code, err := readEntry(reader)
			if err != nil {
				return nil, err
			}
			if code != "" && !seen[code] {
				codes = append(codes, code)
				seen[code] = true
			}
		} else {
			if err := skipField(reader, wireType); err != nil {
				return nil, err
			}
		}
	}

	return codes, nil
}

func readEntry(reader *bytes.Reader) (string, error) {
	length, err := readVarint(reader)
	if err != nil {
		return "", err
	}

	msgData := make([]byte, length)
	if _, err := io.ReadFull(reader, msgData); err != nil {
		return "", err
	}

	return parseCountryCode(msgData), nil
}

func parseCountryCode(msgData []byte) string {
	reader := bytes.NewReader(msgData)

	for reader.Len() > 0 {
		tag, err := readVarint(reader)
		if err != nil {
			break
		}

		fieldNumber := tag >> 3
		wireType := tag & 0x07

		if fieldNumber == fieldNumberEntry && wireType == wireTypeLength {
			length, err := readVarint(reader)
			if err != nil {
				break
			}
			codeBytes := make([]byte, length)
			if _, err := io.ReadFull(reader, codeBytes); err != nil {
				break
			}
			return string(codeBytes)
		}

		if err := skipField(reader, wireType); err != nil {
			break
		}
	}

	return ""
}

func skipField(reader *bytes.Reader, wireType uint64) error {
	switch wireType {
	case wireTypeVarint:
		_, err := readVarint(reader)
		return err
	case wireTypeLength:
		length, err := readVarint(reader)
		if err != nil {
			return err
		}
		_, err = io.CopyN(io.Discard, reader, int64(length))
		return err
	case wireTypeFixed32:
		_, err := io.CopyN(io.Discard, reader, 4)
		return err
	case wireTypeFixed64:
		_, err := io.CopyN(io.Discard, reader, 8)
		return err
	default:
		return fmt.Errorf("unknown wire type: %d", wireType)
	}
}

func marshalEmptyEntry(countryCode string) []byte {
	var buf bytes.Buffer
	writeStringField(&buf, fieldNumberEntry, countryCode)
	return buf.Bytes()
}

func writeField(buf *bytes.Buffer, fieldNum int, wireType int, data []byte) {
	writeVarint(buf, uint64((fieldNum<<3)|wireType))
	writeVarint(buf, uint64(len(data)))
	buf.Write(data)
}

func writeStringField(buf *bytes.Buffer, fieldNum int, value string) {
	writeVarint(buf, uint64((fieldNum<<3)|wireTypeLength))
	writeVarint(buf, uint64(len(value)))
	buf.WriteString(value)
}

func readVarint(reader *bytes.Reader) (uint64, error) {
	var result uint64
	var shift uint

	for {
		b, err := reader.ReadByte()
		if err != nil {
			return 0, err
		}

		result |= uint64(b&0x7f) << shift
		if b&0x80 == 0 {
			return result, nil
		}
		shift += 7
		if shift >= 64 {
			return 0, errors.New("varint too long")
		}
	}
}

func writeVarint(buf *bytes.Buffer, value uint64) {
	for value >= 0x80 {
		buf.WriteByte(byte(value&0x7f) | 0x80)
		value >>= 7
	}
	buf.WriteByte(byte(value))
}

func getTestCategoryName(fileType string) string {
	if fileType == "geoip" {
		return "TRIMCATIP"
	}
	return "TRIMCATSITE"
}
