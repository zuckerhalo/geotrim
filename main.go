package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/zuckerhalo/geotrim/lib"
)

func main() {
	if len(os.Args) < 2 {
		os.Exit(1)
	}

	switch os.Args[1] {
	case "extract":
		handleExtract()
	case "create":
		handleCreate()
	case "trim":
		handleTrim()
	default:
		os.Exit(1)
	}
}

func handleExtract() {
	fs := flag.NewFlagSet("extract", flag.ExitOnError)
	input := fs.String("input", "", "")
	outputDir := fs.String("output", "data", "")
	typeStr := fs.String("type", "", "")
	addTest := fs.Bool("add-test", true, "")

	fs.Parse(os.Args[2:])

	if *input == "" || *typeStr == "" {
		os.Exit(1)
	}

	t := strings.ToLower(*typeStr)
	if t != "geoip" && t != "geosite" {
		os.Exit(1)
	}

	config := lib.ExtractorConfig{
		InputFile:  *input,
		OutputDir:  filepath.Join(*outputDir, "data"+t),
		Type:       t,
		AddTestCat: *addTest,
	}

	if _, err := lib.NewExtractor(config).Extract(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func handleCreate() {
	fs := flag.NewFlagSet("create", flag.ExitOnError)
	dataDir := fs.String("data", "", "")
	outputName := fs.String("output", "", "")
	outputDir := fs.String("outdir", "./", "")
	typeStr := fs.String("type", "", "")

	fs.Parse(os.Args[2:])

	if *dataDir == "" || *outputName == "" || *typeStr == "" {
		os.Exit(1)
	}

	t := strings.ToLower(*typeStr)
	if t != "geoip" && t != "geosite" {
		os.Exit(1)
	}

	config := lib.CreatorConfig{
		DataDir:    *dataDir,
		OutputName: *outputName,
		OutputDir:  *outputDir,
		Type:       t,
	}

	if err := lib.NewCreator(config).Create(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func handleTrim() {
	fs := flag.NewFlagSet("trim", flag.ExitOnError)
	input := fs.String("input", "", "")
	output := fs.String("output", "", "")
	outputDir := fs.String("outdir", "./", "")
	typeStr := fs.String("type", "", "")
	dataDir := fs.String("data", "", "")

	fs.Parse(os.Args[2:])

	if *input == "" || *output == "" || *typeStr == "" {
		os.Exit(1)
	}

	t := strings.ToLower(*typeStr)
	if t != "geoip" && t != "geosite" {
		os.Exit(1)
	}

	var categories []string
	if *dataDir != "" {
		entries, err := os.ReadDir(*dataDir)
		if err != nil {
			os.Exit(1)
		}
		for _, e := range entries {
			if !e.IsDir() {
				categories = append(categories, e.Name())
			}
		}
	}

	config := lib.TrimmerConfig{
		InputFile:  *input,
		OutputName: *output,
		OutputDir:  *outputDir,
		Type:       t,
		Categories: categories,
	}

	if err := lib.NewTrimmer(config).Trim(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
