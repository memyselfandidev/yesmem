package daemon

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

const aiJailRepo = "akitaonrails/ai-jail"

func downloadAiJail() (string, error) {
	if os.Getenv("YESMEM_DISABLE_AIJAIL_DOWNLOAD") != "" {
		return "", errors.New("ai-jail download disabled by YESMEM_DISABLE_AIJAIL_DOWNLOAD env")
	}
	installDir := filepath.Join(os.Getenv("HOME"), ".local", "bin")
	destPath := filepath.Join(installDir, "ai-jail")

	if info, err := os.Stat(destPath); err == nil && info.Mode().IsRegular() {
		return destPath, nil
	}

	assetName, err := aiJailAssetName(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return "", err
	}
	url, err := resolveAiJailAssetURL(assetName)
	if err != nil {
		return "", fmt.Errorf("resolve release: %w", err)
	}

	log.Printf("[sandbox] downloading ai-jail from %s", url)
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("download failed: %s", resp.Status)
	}

	_ = os.MkdirAll(installDir, 0o755)
	tmp := destPath + ".tmp"
	if err := extractAiJailFromTarGz(resp.Body, tmp); err != nil {
		os.Remove(tmp)
		return "", fmt.Errorf("extract ai-jail: %w", err)
	}

	if err := os.Rename(tmp, destPath); err != nil {
		os.Remove(tmp)
		return "", err
	}

	log.Printf("[sandbox] ai-jail installed to %s", destPath)
	return destPath, nil
}

func resolveAiJailAssetURL(assetName string) (string, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", aiJailRepo)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(apiURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var release struct {
		Assets []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}

	for _, a := range release.Assets {
		if a.Name == assetName || a.Name == assetName+".tar.gz" || a.Name == assetName+".zip" {
			return a.BrowserDownloadURL, nil
		}
	}
	return "", fmt.Errorf("asset %q not found in latest release", assetName)
}

// aiJailAssetName maps a Go runtime.GOOS/GOARCH pair to the actual asset
// filename published by https://github.com/akitaonrails/ai-jail. Upstream uses
// macos (not darwin) and x86_64/aarch64 (not amd64/arm64). Combinations not
// shipped upstream return an explicit error so the daemon falls back to the
// unsandboxed path with a clear log line instead of a silent 404.
func aiJailAssetName(goos, goarch string) (string, error) {
	var osName string
	switch goos {
	case "linux":
		osName = "linux"
	case "darwin":
		osName = "macos"
	default:
		return "", fmt.Errorf("ai-jail: unsupported OS %q (only linux and darwin are released)", goos)
	}
	var archName string
	switch goarch {
	case "amd64":
		archName = "x86_64"
	case "arm64":
		archName = "aarch64"
	default:
		return "", fmt.Errorf("ai-jail: unsupported arch %q (only amd64 and arm64 are released)", goarch)
	}
	return fmt.Sprintf("ai-jail-%s-%s.tar.gz", osName, archName), nil
}

// extractAiJailFromTarGz reads a gzip-compressed tar archive from r, finds the
// first regular file whose basename is "ai-jail" (the upstream tarball ships it
// flat at the root, but a nested layout is tolerated), and writes it to
// destPath with mode 0o755. Caller is responsible for the atomic rename.
func extractAiJailFromTarGz(r io.Reader, destPath string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("gzip: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("tar: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		if filepath.Base(hdr.Name) != "ai-jail" {
			continue
		}
		f, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
		if err != nil {
			return err
		}
		if _, err := io.Copy(f, tr); err != nil {
			f.Close()
			return err
		}
		return f.Close()
	}
	return errors.New("ai-jail binary not found in tarball")
}
