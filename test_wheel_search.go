package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// PyPIResponse represents the response from PyPI JSON API
type PyPIResponse struct {
	Info     PyPIPackageInfo            `json:"info"`
	Releases map[string][]PyPIFile      `json:"releases"`
}

// PyPIPackageInfo represents package information from PyPI
type PyPIPackageInfo struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Summary     string `json:"summary"`
	Description string `json:"description"`
}

// PyPIFile represents a file in PyPI release
type PyPIFile struct {
	Filename string `json:"filename"`
	URL      string `json:"url"`
	Size     int64  `json:"size"`
	Digests  struct {
		SHA256 string `json:"sha256"`
	} `json:"digests"`
	UploadTime string `json:"upload_time"`
	FileType   string `json:"packagetype"`
}

// PyPIWheelInfo represents wheel file information from PyPI
type PyPIWheelInfo struct {
	Filename string
	URL      string
	Version  string
	Platform string
	Arch     string
	Size     int64
	Digest   string
}

func main() {
	libraryName := "numpy"
	url := fmt.Sprintf("https://pypi.org/pypi/%s/json", libraryName)
	
	fmt.Printf("Searching PyPI for %s...\n", libraryName)
	
	resp, err := http.Get(url)
	if err != nil {
		fmt.Printf("Error: failed to fetch PyPI data: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Error: PyPI API returned status %d\n", resp.StatusCode)
		return
	}

	var pypiResp PyPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&pypiResp); err != nil {
		fmt.Printf("Error: failed to decode PyPI response: %v\n", err)
		return
	}

	fmt.Printf("Found package: %s\n", pypiResp.Info.Name)
	fmt.Printf("Latest version: %s\n", pypiResp.Info.Version)

	// Find wheel files for the latest version
	latestVersion := pypiResp.Info.Version
	files, exists := pypiResp.Releases[latestVersion]
	if !exists {
		fmt.Printf("Error: no files found for version %s\n", latestVersion)
		return
	}

	fmt.Printf("\nWheel files for version %s:\n", latestVersion)
	var bestWheel *PyPIWheelInfo
	
	for _, file := range files {
		if file.FileType == "bdist_wheel" {
			wheelInfo := parseWheelFilename(file.Filename)
			if wheelInfo != nil {
				wheelInfo.URL = file.URL
				wheelInfo.Size = file.Size
				wheelInfo.Digest = file.Digests.SHA256
				wheelInfo.Version = latestVersion
				
				fmt.Printf("  - %s (Platform: %s, Arch: %s, Size: %.1f MB)\n", 
					wheelInfo.Filename, wheelInfo.Platform, wheelInfo.Arch, 
					float64(wheelInfo.Size)/(1024*1024))
				
				// Select the best wheel for macOS x86_64
				if wheelInfo.Platform == "macos" && wheelInfo.Arch == "x86_64" {
					bestWheel = wheelInfo
				}
			}
		}
	}

	if bestWheel != nil {
		fmt.Printf("\nSelected best wheel for macOS x86_64:\n")
		fmt.Printf("  Filename: %s\n", bestWheel.Filename)
		fmt.Printf("  URL: %s\n", bestWheel.URL)
		fmt.Printf("  Size: %.1f MB\n", float64(bestWheel.Size)/(1024*1024))
		fmt.Printf("  SHA256: %s\n", bestWheel.Digest)
	} else {
		fmt.Printf("\nNo suitable wheel found for macOS x86_64\n")
	}
}

// parseWheelFilename parses wheel filename to extract platform and architecture
func parseWheelFilename(filename string) *PyPIWheelInfo {
	// Example: numpy-1.24.3-cp312-cp312-macosx_10_9_x86_64.whl
	parts := strings.Split(filename, "-")
	if len(parts) < 4 {
		return nil
	}

	// Extract platform and architecture from the last part
	platformPart := strings.TrimSuffix(parts[len(parts)-1], ".whl")
	
	platform := getWheelPlatform(platformPart)
	arch := getWheelArch(platformPart)

	return &PyPIWheelInfo{
		Filename: filename,
		Platform: platform,
		Arch:     arch,
	}
}

// getWheelPlatform extracts platform from wheel filename
func getWheelPlatform(platformPart string) string {
	if strings.Contains(platformPart, "macosx") {
		return "macos"
	} else if strings.Contains(platformPart, "linux") {
		return "linux"
	} else if strings.Contains(platformPart, "win") {
		return "windows"
	}
	return "any"
}

// getWheelArch extracts architecture from wheel filename
func getWheelArch(platformPart string) string {
	if strings.Contains(platformPart, "x86_64") || strings.Contains(platformPart, "amd64") {
		return "x86_64"
	} else if strings.Contains(platformPart, "aarch64") || strings.Contains(platformPart, "arm64") {
		return "aarch64"
	}
	return "any"
} 