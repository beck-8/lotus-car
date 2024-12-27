package index

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/minerdao/lotus-car/util"
	"github.com/urfave/cli/v2"
)

func Command() *cli.Command {
	return &cli.Command{
		Name:  "index",
		Usage: "Index all files in target directory and save to json file",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "source-dir",
				Usage:    "Source directory to index",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "output-dir",
				Usage:    "Output directory to save index file",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "index-file",
				Usage:    "Index file name",
				Required: true,
			},
		},
		Action: func(c *cli.Context) error {
			sourceDir := c.String("source-dir")
			outputDir := c.String("output-dir")
			indexFile := c.String("index-file")

			// Check if directories exist
			if _, err := os.Stat(sourceDir); os.IsNotExist(err) {
				return fmt.Errorf("source directory does not exist: %s", sourceDir)
			}
			if _, err := os.Stat(outputDir); os.IsNotExist(err) {
				return fmt.Errorf("parent directory does not exist: %s", outputDir)
			}

			// List all files
			var files []string
			err := filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if !info.IsDir() {
					files = append(files, path)
				}
				return nil
			})
			if err != nil {
				return fmt.Errorf("error walking through directory: %v", err)
			}

			// Create JSON array
			var jsonArr []map[string]interface{}
			for _, file := range files {
				info, err := os.Stat(file)
				if err != nil {
					return fmt.Errorf("error getting file info: %v", err)
				}

				fileInfo := map[string]interface{}{
					"Path": file,
					"Size": info.Size(),
				}
				jsonArr = append(jsonArr, fileInfo)
				fmt.Printf("Indexed: %s, size: %s\n", file, util.FormatSize(info.Size()))
			}

			// Write to JSON file
			outputFile := filepath.Join(outputDir, indexFile)

			// Remove existing file if exists
			if _, err := os.Stat(outputFile); err == nil {
				fmt.Printf("Removing existing index file: %s\n", outputFile)
				err = os.Remove(outputFile)
				if err != nil {
					return fmt.Errorf("error removing existing index file: %v", err)
				}
			}

			// Create parent directory if not exists
			if err := os.MkdirAll(outputDir, 0755); err != nil {
				return fmt.Errorf("error creating output directory: %v", err)
			}

			// Write JSON data
			fmt.Printf("Writing index file: %s\n", outputFile)
			jsonData, err := json.MarshalIndent(jsonArr, "", "    ")
			if err != nil {
				return fmt.Errorf("error marshaling JSON: %v", err)
			}

			err = os.WriteFile(outputFile, jsonData, 0644)
			if err != nil {
				return fmt.Errorf("error writing index file: %v", err)
			}

			fmt.Printf("Successfully indexed %d files: %s\n", len(jsonArr), outputFile)
			return nil
		},
	}
}
