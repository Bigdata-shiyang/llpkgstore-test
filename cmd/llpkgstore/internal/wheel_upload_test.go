package internal

import (
	"testing"
)

func TestParseWheelFilename(t *testing.T) {
	tests := []struct {
		filename string
		platform string
		arch     string
	}{
		{
			filename: "numpy-1.24.3-cp312-cp312-macosx_10_9_x86_64.whl",
			platform: "macos",
			arch:     "x86_64",
		},
		{
			filename: "pandas-2.0.3-cp312-cp312-linux_x86_64.whl",
			platform: "linux",
			arch:     "x86_64",
		},
		{
			filename: "scipy-1.11.1-cp312-cp312-win_amd64.whl",
			platform: "windows",
			arch:     "x86_64",
		},
		{
			filename: "requests-2.31.0-py3-none-any.whl",
			platform: "any",
			arch:     "any",
		},
	}

	uploader := &WheelUploader{}

	for _, test := range tests {
		platform, arch, pythonVersion := uploader.parseWheelFilename(test.filename)
		if platform != test.platform {
			t.Errorf("parseWheelFilename(%s) platform = %s, want %s", test.filename, platform, test.platform)
		}
		if arch != test.arch {
			t.Errorf("parseWheelFilename(%s) arch = %s, want %s", test.filename, arch, test.arch)
		}
		// Log Python version for debugging
		fmt.Printf("parseWheelFilename(%s) = platform:%s, arch:%s, python:%s\n", 
			test.filename, platform, arch, pythonVersion)
	}
}

func TestGetWheelPlatform(t *testing.T) {
	tests := []struct {
		filename string
		platform string
	}{
		{
			filename: "numpy-1.24.3-cp312-cp312-macosx_10_9_x86_64.whl",
			platform: "macosx_10_9_x86_64",
		},
		{
			filename: "pandas-2.0.3-cp312-cp312-linux_x86_64.whl",
			platform: "linux_x86_64",
		},
		{
			filename: "requests-2.31.0-py3-none-any.whl",
			platform: "any",
		},
	}

	uploader := &WheelUploader{}

	for _, test := range tests {
		platform := uploader.getWheelPlatform(test.filename)
		if platform != test.platform {
			t.Errorf("getWheelPlatform(%s) = %s, want %s", test.filename, platform, test.platform)
		}
	}
}

func TestIsBetterWheel(t *testing.T) {
	tests := []struct {
		new     PyPIFile
		current PyPIFile
		better  bool
	}{
		{
			new: PyPIFile{
				Filename: "numpy-1.24.3-cp312-cp312-macosx_10_9_x86_64.whl",
			},
			current: PyPIFile{
				Filename: "numpy-1.24.3-py3-none-any.whl",
			},
			better: true, // Platform-specific is better than universal
		},
		{
			new: PyPIFile{
				Filename: "numpy-1.24.3-py3-none-any.whl",
			},
			current: PyPIFile{
				Filename: "numpy-1.24.3-cp312-cp312-macosx_10_9_x86_64.whl",
			},
			better: false, // Universal is not better than platform-specific
		},
	}

	uploader := &WheelUploader{}

	for _, test := range tests {
		better := uploader.isBetterWheel(test.new, test.current)
		if better != test.better {
			t.Errorf("isBetterWheel(%s, %s) = %t, want %t", test.new.Filename, test.current.Filename, better, test.better)
		}
	}
} 