package config

import (
	"os"
	"runtime"
<<<<<<< HEAD
=======
	"strings"
>>>>>>> test-wheel-upload-v2
)

// WheelConfig holds configuration for wheel upload functionality
type WheelConfig struct {
	// Target repository for wheel files
	TargetRepoOwner string
	TargetRepoName  string
	
	// PyPI settings
	PyPIBaseURL string
	
	// Platform settings
	SupportedPlatforms []string
	SupportedArchs     []string
	PythonVersion      string
	
	// GitHub settings
	GitHubToken        string
	SourceRepoOwner    string
	SourceRepoName     string
}

// NewWheelConfig creates a new wheel configuration with default values
func NewWheelConfig() *WheelConfig {
	config := &WheelConfig{
		TargetRepoOwner: getEnvOrDefault("TARGET_REPO_OWNER", "Bigdata-shiyang"),
		TargetRepoName:  getEnvOrDefault("TARGET_REPO_NAME", "test"),
		PyPIBaseURL:     getEnvOrDefault("PYPI_BASE_URL", "https://pypi.org/pypi"),
		PythonVersion:   getEnvOrDefault("PYTHON_VERSION", "3.12"),
		GitHubToken:     os.Getenv("GITHUB_TOKEN"),
		SourceRepoOwner: getEnvOrDefault("GITHUB_REPOSITORY_OWNER", "Bigdata-shiyang"),
<<<<<<< HEAD
		SourceRepoName:  getEnvOrDefault("GITHUB_REPOSITORY", "llpkgstore-test"),
=======
		SourceRepoName:  getRepoNameFromFullPath(getEnvOrDefault("GITHUB_REPOSITORY", "llpkgstore-test")),
>>>>>>> test-wheel-upload-v2
	}
	
	// Set supported platforms and architectures
	config.SupportedPlatforms = []string{"macos", "linux", "windows"}
	config.SupportedArchs = []string{"x86_64", "aarch64"}
	
	return config
}

// GetCurrentPlatform returns the current platform
func (c *WheelConfig) GetCurrentPlatform() string {
	switch runtime.GOOS {
	case "darwin":
		return "macos"
	case "linux":
		return "linux"
	case "windows":
		return "windows"
	default:
		return "any"
	}
}

// GetCurrentArch returns the current architecture
func (c *WheelConfig) GetCurrentArch() string {
	switch runtime.GOARCH {
	case "amd64":
		return "x86_64"
	case "arm64":
		return "aarch64"
	default:
		return "any"
	}
}

// IsPlatformSupported checks if a platform is supported
func (c *WheelConfig) IsPlatformSupported(platform string) bool {
	if platform == "any" {
		return true
	}
	
	for _, supported := range c.SupportedPlatforms {
		if platform == supported {
			return true
		}
	}
	return false
}

// IsArchSupported checks if an architecture is supported
func (c *WheelConfig) IsArchSupported(arch string) bool {
	if arch == "any" {
		return true
	}
	
	for _, supported := range c.SupportedArchs {
		if arch == supported {
			return true
		}
	}
	return false
}

// getEnvOrDefault gets an environment variable or returns a default value
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
<<<<<<< HEAD
=======
}

// getRepoNameFromFullPath extracts repository name from full path (owner/repo)
func getRepoNameFromFullPath(fullPath string) string {
	parts := strings.Split(fullPath, "/")
	if len(parts) >= 2 {
		return parts[len(parts)-1]
	}
	return fullPath
>>>>>>> test-wheel-upload-v2
} 