package capfile

import (
	"fmt"
	"os"
	"path/filepath"
)

func ScanDirectory(dir string) ([]CapFile, []error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, []error{fmt.Errorf("read dir %s: %w", dir, err)}
	}

	var caps []CapFile
	var errs []error

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		capPath := filepath.Join(dir, entry.Name(), "CAP.md")
		data, err := os.ReadFile(capPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			errs = append(errs, fmt.Errorf("read %s: %w", capPath, err))
			continue
		}
		cf, err := Parse(data)
		if err != nil {
			errs = append(errs, fmt.Errorf("parse %s: %w", capPath, err))
			continue
		}
		cf.SourcePath = capPath
		caps = append(caps, *cf)
	}
	return caps, errs
}

func ScanAll(userDir, projectDir string) ([]CapFile, []error) {
	var allCaps []CapFile
	var allErrs []error

	caps, errs := ScanDirectory(userDir)
	allCaps = append(allCaps, caps...)
	allErrs = append(allErrs, errs...)

	caps, errs = ScanDirectory(projectDir)
	allCaps = append(allCaps, caps...)
	allErrs = append(allErrs, errs...)

	return allCaps, allErrs
}
