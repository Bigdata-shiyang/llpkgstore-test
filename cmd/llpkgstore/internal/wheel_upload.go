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
	"sort"
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
	client        *github.Client
	config        *config.WheelConfig
}

// PyPIResponse represents the response from PyPI JSON API
type PyPIResponse struct {
	Info     PyPIPackageInfo `json:"info"`
	Releases map[string][]PyPIFile `json:"releases"`
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
	Filename     string
	URL          string
	Version      string
	Platform     string
	Arch         string
	PythonVersion string
	Size         int64
	Digest       string
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

	// Debug: Print configuration
	fmt.Printf("Debug: SourceRepoOwner = %s\n", w.config.SourceRepoOwner)
	fmt.Printf("Debug: SourceRepoName = %s\n", w.config.SourceRepoName)
	fmt.Printf("Debug: PR Number = %s\n", prNumber)

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

// searchPyPI searches PyPI for the library and returns wheel info
func (w *WheelUploader) searchPyPI(libraryName string) (*PyPIWheelInfo, error) {
	// PyPI JSON API endpoint
	url := fmt.Sprintf("%s/%s/json", w.config.PyPIBaseURL, libraryName)
	
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("PyPI API returned status: %d", resp.StatusCode)
	}

	// Parse JSON response
	var pypiResp PyPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&pypiResp); err != nil {
		return nil, fmt.Errorf("failed to parse PyPI response: %v", err)
	}

	// Get the latest version
	latestVersion := pypiResp.Info.Version
	if latestVersion == "" {
		// Find the latest version from releases
		var versions []string
		for version := range pypiResp.Releases {
			versions = append(versions, version)
		}
		if len(versions) == 0 {
			return nil, fmt.Errorf("no versions found for library %s", libraryName)
		}
		sort.Strings(versions)
		latestVersion = versions[len(versions)-1]
	}

	// Get files for the latest version
	files, exists := pypiResp.Releases[latestVersion]
	if !exists {
		return nil, fmt.Errorf("no files found for version %s", latestVersion)
	}

	// Find the best matching wheel file
	var bestWheel *PyPIFile
	fmt.Printf("Available wheel files for %s version %s:\n", libraryName, latestVersion)
	
	for i, file := range files {
		if file.FileType == "bdist_wheel" && strings.HasSuffix(file.Filename, ".whl") {
			platform, arch, pythonVersion := w.parseWheelFilename(file.Filename)
			fmt.Printf("  [%d] %s (Platform: %s, Arch: %s, Python: %s)\n", 
				i+1, file.Filename, platform, arch, pythonVersion)
			
			if bestWheel == nil || w.isBetterWheel(file, *bestWheel) {
				bestWheel = &file
				fmt.Printf("    -> Selected as best wheel\n")
			}
		}
	}

	if bestWheel == nil {
		return nil, fmt.Errorf("no wheel files found for library %s version %s", libraryName, latestVersion)
	}

	// Parse wheel filename to extract platform, architecture, and Python version
	platform, arch, pythonVersion := w.parseWheelFilename(bestWheel.Filename)

	return &PyPIWheelInfo{
		Filename:     bestWheel.Filename,
		URL:          bestWheel.URL,
		Version:      latestVersion,
		Platform:     platform,
		Arch:         arch,
		PythonVersion: pythonVersion,
		Size:         bestWheel.Size,
		Digest:       bestWheel.Digests.SHA256,
	}, nil
}

// isBetterWheel determines if one wheel file is better than another
func (w *WheelUploader) isBetterWheel(new, current PyPIFile) bool {
	newPlatform, newArch, newPython := w.parseWheelFilename(new.Filename)
	currentPlatform, currentArch, currentPython := w.parseWheelFilename(current.Filename)
	
	// Get target platform, architecture, and Python version from config
	targetPlatform := w.config.GetCurrentPlatform()
	targetArch := w.config.GetCurrentArch()
	targetPython := w.config.PythonVersion
	
	// Debug logging
	fmt.Printf("Comparing wheels:\n")
	fmt.Printf("  New: %s (Platform: %s, Arch: %s, Python: %s)\n", new.Filename, newPlatform, newArch, newPython)
	fmt.Printf("  Current: %s (Platform: %s, Arch: %s, Python: %s)\n", current.Filename, currentPlatform, currentArch, currentPython)
	fmt.Printf("  Target: Platform: %s, Arch: %s, Python: %s\n", targetPlatform, targetArch, targetPython)
	
	// Score-based comparison
	newScore := w.getWheelScore(newPlatform, newArch, newPython, targetPlatform, targetArch, targetPython)
	currentScore := w.getWheelScore(currentPlatform, currentArch, currentPython, targetPlatform, targetArch, targetPython)
	
	fmt.Printf("  Scores: New=%d, Current=%d\n", newScore, currentScore)
	
	return newScore > currentScore
}

// getWheelScore calculates a score for wheel compatibility
func (w *WheelUploader) getWheelScore(platform, arch, pythonVersion, targetPlatform, targetArch, targetPython string) int {
	score := 0
	
	// Python version matching (highest priority - 200 points)
	if pythonVersion == targetPython {
		score += 200
	} else if pythonVersion == "any" {
		score += 20
	} else if targetPython == "any" {
		score += 100
	}
	
	// Platform matching (second priority - 100 points)
	if platform == targetPlatform {
		score += 100
	} else if platform == "any" {
		score += 10
	} else if targetPlatform == "any" {
		score += 50
	}
	
	// Architecture matching (third priority - 50 points)
	if arch == targetArch {
		score += 50
	} else if arch == "any" {
		score += 5
	} else if targetArch == "any" {
		score += 25
	}
	
	// Prefer specific Python versions over universal
	if pythonVersion != "any" {
		score += 20
	}
	
	// Prefer specific platforms over universal
	if platform != "any" {
		score += 10
	}
	
	// Prefer specific architectures over universal
	if arch != "any" {
		score += 5
	}
	
	return score
}

// getWheelPlatform extracts platform information from wheel filename
func (w *WheelUploader) getWheelPlatform(filename string) string {
	// Wheel filename format: package-version-python_tag-platform_tag.whl
	parts := strings.Split(filename, "-")
	if len(parts) < 4 {
		return "any"
	}
	
	platformPart := parts[len(parts)-1]
	platformPart = strings.TrimSuffix(platformPart, ".whl")
	
	if platformPart == "any" || strings.Contains(platformPart, "py3") {
		return "any"
	}
	
	return platformPart
}

// parseWheelFilename parses wheel filename to extract platform, architecture, and Python version
func (w *WheelUploader) parseWheelFilename(filename string) (platform, arch, pythonVersion string) {
	// Wheel filename format: package-version-python_tag-platform_tag.whl
	parts := strings.Split(filename, "-")
	if len(parts) < 4 {
		return "any", "any", "any"
	}
	
	// Extract Python version from python_tag (e.g., cp312, py3, py39)
	pythonTag := parts[len(parts)-2]
	if strings.HasPrefix(pythonTag, "cp") {
		// Extract version from cp312 -> 3.12
		versionStr := strings.TrimPrefix(pythonTag, "cp")
		if len(versionStr) >= 2 {
			pythonVersion = versionStr[:1] + "." + versionStr[1:]
		}
	} else if strings.HasPrefix(pythonTag, "py") {
		// Extract version from py312 -> 3.12
		versionStr := strings.TrimPrefix(pythonTag, "py")
		if len(versionStr) >= 2 {
			pythonVersion = versionStr[:1] + "." + versionStr[1:]
		}
	}
	
	platformPart := parts[len(parts)-1]
	platformPart = strings.TrimSuffix(platformPart, ".whl")
	
	if platformPart == "any" {
		return "any", "any", pythonVersion
	}
	
	// Parse platform and architecture
	// Examples: macosx_10_9_x86_64, linux_x86_64, win_amd64, manylinux_2_27_x86_64
	if strings.Contains(platformPart, "macosx") {
		platform = "macos"
		if strings.Contains(platformPart, "x86_64") {
			arch = "x86_64"
		} else if strings.Contains(platformPart, "aarch64") {
			arch = "aarch64"
		}
	} else if strings.Contains(platformPart, "manylinux") || strings.Contains(platformPart, "linux") {
		platform = "linux"
		if strings.Contains(platformPart, "x86_64") {
			arch = "x86_64"
		} else if strings.Contains(platformPart, "aarch64") {
			arch = "aarch64"
		}
	} else if strings.Contains(platformPart, "win") {
		platform = "windows"
		if strings.Contains(platformPart, "amd64") {
			arch = "x86_64"
		}
	} else {
		platform = "any"
		arch = "any"
	}
	
	return platform, arch, pythonVersion
}

// downloadWheel downloads the wheel file from PyPI
func (w *WheelUploader) downloadWheel(wheelInfo *PyPIWheelInfo) (string, error) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "wheel-download")
	if err != nil {
		return "", err
	}

	wheelPath := filepath.Join(tempDir, wheelInfo.Filename)
	
	// Download the wheel file
	resp, err := http.Get(wheelInfo.URL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download wheel: %d", resp.StatusCode)
	}

	file, err := os.Create(wheelPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return "", err
	}

	return wheelPath, nil
}

// createOrUpdateRelease creates or updates a GitHub Release
func (w *WheelUploader) createOrUpdateRelease(libraryName, version string) (*github.RepositoryRelease, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	releaseTag := fmt.Sprintf("%s/v%s", libraryName, version)
	releaseName := fmt.Sprintf("%s/v%s", libraryName, version)

	// Check if release already exists
	release, _, err := w.client.Repositories.GetReleaseByTag(ctx, w.config.TargetRepoOwner, w.config.TargetRepoName, releaseTag)
	if err == nil {
		// Release exists, update it
		release.Name = &releaseName
		release.Body = github.String(fmt.Sprintf("Wheel files for %s version %s", libraryName, version))
		
		updatedRelease, _, err := w.client.Repositories.EditRelease(ctx, w.config.TargetRepoOwner, w.config.TargetRepoName, *release.ID, release)
		if err != nil {
			return nil, err
		}
		return updatedRelease, nil
	}

	// Create new release
	newRelease := &github.RepositoryRelease{
		TagName: &releaseTag,
		Name:    &releaseName,
		Body:    github.String(fmt.Sprintf("Wheel files for %s version %s", libraryName, version)),
	}

	createdRelease, _, err := w.client.Repositories.CreateRelease(ctx, w.config.TargetRepoOwner, w.config.TargetRepoName, newRelease)
	if err != nil {
		return nil, err
	}

	return createdRelease, nil
}

// uploadWheelToRelease uploads the wheel file to the GitHub Release
func (w *WheelUploader) uploadWheelToRelease(release *github.RepositoryRelease, wheelPath, filename string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Check if asset already exists
	assets, _, err := w.client.Repositories.ListReleaseAssets(ctx, w.config.TargetRepoOwner, w.config.TargetRepoName, *release.ID, &github.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list release assets: %v", err)
	}

	// Check if file already exists
	for _, asset := range assets {
		if *asset.Name == filename {
			fmt.Printf("Asset %s already exists in release, skipping upload\n", filename)
			return nil
		}
	}

	file, err := os.Open(wheelPath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Upload the asset
	_, _, err = w.client.Repositories.UploadReleaseAsset(ctx, w.config.TargetRepoOwner, w.config.TargetRepoName, *release.ID, &github.UploadOptions{
		Name: filename,
	}, file)
	
	return err
}

// updatePRStatus updates the PR with success status and information
func (w *WheelUploader) updatePRStatus(prNumber string, libraryName string, wheelInfo *PyPIWheelInfo, release *github.RepositoryRelease) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Convert string to int for GitHub API
	prNum, err := strconv.Atoi(prNumber)
	if err != nil {
		return fmt.Errorf("invalid PR number: %s", prNumber)
	}

	comment := fmt.Sprintf(`## ✅ Wheel Upload Successful

**Library**: %s
**Version**: %s
**Python Version**: %s
**Platform**: %s
**Architecture**: %s
**File Size**: %d bytes
**SHA256**: %s

**Release**: [%s](%s)

The wheel file has been successfully uploaded to GitHub Release. You can now use:

`+"```bash"+`
llgo get %s
`+"```"+`

The wheel file is now available in the release and will be automatically downloaded when needed.`, 
		libraryName, wheelInfo.Version, wheelInfo.PythonVersion, wheelInfo.Platform, wheelInfo.Arch, 
		wheelInfo.Size, wheelInfo.Digest, *release.TagName, *release.HTMLURL, libraryName)

	_, _, err = w.client.Issues.CreateComment(ctx, w.config.SourceRepoOwner, w.config.SourceRepoName, prNum, &github.IssueComment{
		Body: &comment,
	})

	return err
}

func init() {
	rootCmd.AddCommand(wheelUploadCmd)
} 