package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-github/v69/github"
	"github.com/goplus/llpkgstore/config"
	"github.com/spf13/cobra"
)

// wheelUploadCmd represents the wheel upload command
var wheelUploadCmd = &cobra.Command{
	Use:   "wheel-upload [PR_NUMBER]",
	Short: "Automatically upload wheel files from PyPI to GitHub Release based on PR",
	Long: `Automatically process PR requests for missing wheel files.
This command will:
1. Parse PR title to extract library name
2. Search PyPI for the library
3. Download appropriate wheel file
4. Create/update GitHub Release
5. Upload wheel file to the release
6. Update PR with status`,
	Args: cobra.ExactArgs(1),
	RunE: processWheelUpload,
}

// WheelUploader handles the wheel upload process
type WheelUploader struct {
	client *github.Client
	config *config.WheelConfig
}

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

// NewWheelUploader creates a new wheel uploader instance
func NewWheelUploader() (*WheelUploader, error) {
	cfg := config.NewWheelConfig()
	
	if cfg.GitHubToken == "" {
		return nil, fmt.Errorf("GITHUB_TOKEN environment variable is required")
	}

	return &WheelUploader{
		client: github.NewClient(nil).WithAuthToken(cfg.GitHubToken),
		config: cfg,
	}, nil
}

// processWheelUpload handles the main wheel upload workflow
func processWheelUpload(cmd *cobra.Command, args []string) error {
	prNumber := args[0]
	
	uploader, err := NewWheelUploader()
	if err != nil {
		return fmt.Errorf("failed to create wheel uploader: %v", err)
	}

	fmt.Printf("Processing PR #%s for wheel upload...\n", prNumber)

	// 1. Get PR information
	libraryName, err := uploader.getPRInfo(prNumber)
	if err != nil {
		return fmt.Errorf("failed to get PR info: %v", err)
	}

	fmt.Printf("Library name extracted: %s\n", libraryName)

	// 2. Search PyPI for the library
	wheelInfo, err := uploader.searchPyPI(libraryName)
	if err != nil {
		return fmt.Errorf("failed to search PyPI: %v", err)
	}

	fmt.Printf("Found wheel: %s\n", wheelInfo.Filename)

	// 3. Download wheel file
	wheelPath, err := uploader.downloadWheel(wheelInfo)
	if err != nil {
		return fmt.Errorf("failed to download wheel: %v", err)
	}

	fmt.Printf("Downloaded wheel to: %s\n", wheelPath)

	// 4. Create/update GitHub Release
	release, err := uploader.createOrUpdateRelease(libraryName, wheelInfo.Version)
	if err != nil {
		return fmt.Errorf("failed to create/update release: %v", err)
	}

	fmt.Printf("Release created/updated: %s\n", *release.TagName)

	// 5. Upload wheel file to release
	err = uploader.uploadWheelToRelease(release, wheelPath, wheelInfo.Filename)
	if err != nil {
		return fmt.Errorf("failed to upload wheel to release: %v", err)
	}

	fmt.Printf("Wheel uploaded successfully to release\n")

	// 6. Update PR with success status
	err = uploader.updatePRStatus(prNumber, libraryName, wheelInfo, release)
	if err != nil {
		return fmt.Errorf("failed to update PR status: %v", err)
	}

	fmt.Printf("PR status updated successfully\n")
	return nil
}

// getPRInfo extracts library name from PR title
func (w *WheelUploader) getPRInfo(prNumber string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Convert string to int for GitHub API
	prNum, err := strconv.Atoi(prNumber)
	if err != nil {
		return "", fmt.Errorf("invalid PR number: %s", prNumber)
	}

	pr, _, err := w.client.PullRequests.Get(ctx, w.config.SourceRepoOwner, w.config.SourceRepoName, prNum)
	if err != nil {
		return "", err
	}

	// Parse PR title to extract library name
	// Expected format: "Add missing wheel: <library_name>"
	title := *pr.Title
	
	// Check if this is a wheel request PR
	if !strings.Contains(strings.ToLower(title), "add missing wheel:") {
		return "", fmt.Errorf("PR title does not match wheel request format. Expected: 'Add missing wheel: <library_name>', got: '%s'", title)
	}
	
	re := regexp.MustCompile(`(?i)add missing wheel:\s*(\w+)`)
	matches := re.FindStringSubmatch(title)
	if len(matches) < 2 {
		return "", fmt.Errorf("PR title does not match expected format: %s", title)
	}

	libraryName := matches[1]
	
	// Validate library name
	if libraryName == "" {
		return "", fmt.Errorf("library name cannot be empty")
	}
	
	// Log the extracted library name
	fmt.Printf("Extracted library name from PR title: %s\n", libraryName)

	return libraryName, nil
}

// searchPyPI searches for wheel files on PyPI
func (w *WheelUploader) searchPyPI(libraryName string) (*PyPIWheelInfo, error) {
	url := fmt.Sprintf("%s/%s/json", w.config.PyPIBaseURL, libraryName)
	
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch PyPI data: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("PyPI API returned status %d", resp.StatusCode)
	}

	var pypiResp PyPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&pypiResp); err != nil {
		return nil, fmt.Errorf("failed to decode PyPI response: %v", err)
	}

	// Find the latest version with wheel files
	var bestWheel *PyPIWheelInfo
	for version, files := range pypiResp.Releases {
		for _, file := range files {
			if file.FileType == "bdist_wheel" {
				wheelInfo := w.parseWheelFilename(file.Filename)
				if wheelInfo != nil && w.isBetterWheel(wheelInfo, bestWheel) {
					wheelInfo.URL = file.URL
					wheelInfo.Size = file.Size
					wheelInfo.Digest = file.Digests.SHA256
					wheelInfo.Version = version
					bestWheel = wheelInfo
				}
			}
		}
	}

	if bestWheel == nil {
		return nil, fmt.Errorf("no suitable wheel file found for %s", libraryName)
	}

	return bestWheel, nil
}

// parseWheelFilename parses wheel filename to extract platform and architecture
func (w *WheelUploader) parseWheelFilename(filename string) *PyPIWheelInfo {
	// Example: numpy-1.24.3-cp312-cp312-macosx_10_9_x86_64.whl
	parts := strings.Split(filename, "-")
	if len(parts) < 4 {
		return nil
	}

	// Extract platform and architecture from the last part
	platformPart := strings.TrimSuffix(parts[len(parts)-1], ".whl")
	
	platform := w.getWheelPlatform(platformPart)
	arch := w.getWheelArch(platformPart)

	return &PyPIWheelInfo{
		Filename: filename,
		Platform: platform,
		Arch:     arch,
	}
}

// getWheelPlatform extracts platform from wheel filename
func (w *WheelUploader) getWheelPlatform(platformPart string) string {
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
func (w *WheelUploader) getWheelArch(platformPart string) string {
	if strings.Contains(platformPart, "x86_64") || strings.Contains(platformPart, "amd64") {
		return "x86_64"
	} else if strings.Contains(platformPart, "aarch64") || strings.Contains(platformPart, "arm64") {
		return "aarch64"
	}
	return "any"
}

// isBetterWheel determines if a wheel is better than the current best
func (w *WheelUploader) isBetterWheel(new, current *PyPIWheelInfo) bool {
	if current == nil {
		return true
	}

	// Prefer current platform
	if w.config.IsPlatformSupported(new.Platform) && !w.config.IsPlatformSupported(current.Platform) {
		return true
	}

	// Prefer current architecture
	if w.config.IsArchSupported(new.Arch) && !w.config.IsArchSupported(current.Arch) {
		return true
	}

	// Prefer exact platform match
	if new.Platform == w.config.GetCurrentPlatform() && current.Platform != w.config.GetCurrentPlatform() {
		return true
	}

	// Prefer exact architecture match
	if new.Arch == w.config.GetCurrentArch() && current.Arch != w.config.GetCurrentArch() {
		return true
	}

	return false
}

// downloadWheel downloads the wheel file to a temporary location
func (w *WheelUploader) downloadWheel(wheelInfo *PyPIWheelInfo) (string, error) {
	resp, err := http.Get(wheelInfo.URL)
	if err != nil {
		return "", fmt.Errorf("failed to download wheel: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	// Create temporary file
	tmpDir := os.TempDir()
	wheelPath := filepath.Join(tmpDir, wheelInfo.Filename)
	
	file, err := os.Create(wheelPath)
	if err != nil {
		return "", fmt.Errorf("failed to create temporary file: %v", err)
	}
	defer file.Close()

	// Copy content
	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to save wheel file: %v", err)
	}

	return wheelPath, nil
}

// createOrUpdateRelease creates or updates a GitHub release
func (w *WheelUploader) createOrUpdateRelease(libraryName, version string) (*github.RepositoryRelease, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tagName := fmt.Sprintf("%s/v%s", libraryName, version)
	releaseName := fmt.Sprintf("%s v%s", libraryName, version)
	body := fmt.Sprintf("Wheel file for %s v%s\n\nAutomatically uploaded by llpkgstore wheel-upload", libraryName, version)

	// Check if release already exists
	releases, _, err := w.client.Repositories.ListReleases(ctx, w.config.TargetRepoOwner, w.config.TargetRepoName, &github.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list releases: %v", err)
	}

	// Find existing release
	for _, release := range releases {
		if *release.TagName == tagName {
			// Update existing release
			release.Name = &releaseName
			release.Body = &body
			
			updatedRelease, _, err := w.client.Repositories.EditRelease(ctx, w.config.TargetRepoOwner, w.config.TargetRepoName, *release.ID, release)
			if err != nil {
				return nil, fmt.Errorf("failed to update release: %v", err)
			}
			return updatedRelease, nil
		}
	}

	// Create new release
	release := &github.RepositoryRelease{
		TagName: &tagName,
		Name:    &releaseName,
		Body:    &body,
		Draft:   github.Bool(false),
	}

	createdRelease, _, err := w.client.Repositories.CreateRelease(ctx, w.config.TargetRepoOwner, w.config.TargetRepoName, release)
	if err != nil {
		return nil, fmt.Errorf("failed to create release: %v", err)
	}

	return createdRelease, nil
}

// uploadWheelToRelease uploads the wheel file to the GitHub release
func (w *WheelUploader) uploadWheelToRelease(release *github.RepositoryRelease, wheelPath, filename string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	file, err := os.Open(wheelPath)
	if err != nil {
		return fmt.Errorf("failed to open wheel file: %v", err)
	}
	defer file.Close()

	// Upload asset
	_, _, err = w.client.Repositories.UploadReleaseAsset(ctx, w.config.TargetRepoOwner, w.config.TargetRepoName, *release.ID, &github.UploadOptions{
		Name: filename,
	}, file)
	if err != nil {
		return fmt.Errorf("failed to upload wheel file: %v", err)
	}

	return nil
}

// updatePRStatus updates the PR with success status and release information
func (w *WheelUploader) updatePRStatus(prNumber, libraryName string, wheelInfo *PyPIWheelInfo, release *github.RepositoryRelease) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	prNum, err := strconv.Atoi(prNumber)
	if err != nil {
		return fmt.Errorf("invalid PR number: %s", prNumber)
	}

	comment := fmt.Sprintf(`✅ Wheel upload completed successfully!

**Release**: %s
**File**: %s
**Size**: %.1f MB
**SHA256**: %s

The wheel file has been successfully uploaded to the release. You can now use this library with llgo.`, 
		*release.HTMLURL, 
		wheelInfo.Filename, 
		float64(wheelInfo.Size)/(1024*1024), 
		wheelInfo.Digest)

	_, _, err = w.client.Issues.CreateComment(ctx, w.config.SourceRepoOwner, w.config.SourceRepoName, prNum, &github.IssueComment{
		Body: &comment,
	})
	if err != nil {
		return fmt.Errorf("failed to create PR comment: %v", err)
	}

	return nil
}

func init() {
	rootCmd.AddCommand(wheelUploadCmd)
} 