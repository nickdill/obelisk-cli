package cmd

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const templateRef = "main"
const maxTemplateBytes = 50 << 20 // 50 MB

var templateHTTPClient = newHTTPClient(60 * time.Second)

func templateTarballURL() string {
	return "https://codeload.github.com/nickdill/obelisk/tar.gz/refs/heads/" + templateRef
}

// applyTemplate downloads the template tarball and writes files into destDir.
// Files in skipPaths are never overwritten. All other existing files are
// skipped unless force is true.
func applyTemplate(destDir string, skipPaths []string, force bool) error {
	skipSet := make(map[string]bool, len(skipPaths))
	for _, p := range skipPaths {
		skipSet[p] = true
	}

	resp, err := templateHTTPClient.Get(templateTarballURL())
	if err != nil {
		return fmt.Errorf("could not download template: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("could not download template: HTTP %d", resp.StatusCode)
	}

	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		parts := strings.SplitN(hdr.Name, "/", 2)
		if len(parts) < 2 || parts[1] == "" {
			continue
		}
		rel := parts[1]
		if strings.HasPrefix(rel, ".git/") || rel == ".git" {
			continue
		}

		full := filepath.Join(destDir, rel)
		if !strings.HasPrefix(full, destDir+string(os.PathSeparator)) {
			return fmt.Errorf("invalid path in archive: %s", hdr.Name)
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(full, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
				return err
			}
			_, statErr := os.Stat(full)
			alreadyExists := statErr == nil

			if alreadyExists && (skipSet[rel] || !force) {
				fmt.Printf("  skip   %s\n", rel)
				continue
			}

			perm := os.FileMode(hdr.Mode).Perm()
			f, err := os.OpenFile(full, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, io.LimitReader(tr, maxTemplateBytes)); err != nil {
				f.Close()
				return err
			}
			f.Close()

			if alreadyExists {
				fmt.Printf("  update %s\n", rel)
			} else {
				fmt.Printf("  create %s\n", rel)
			}
		}
	}
	return nil
}
