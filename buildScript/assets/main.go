// Downloads versioned public routing databases and stores the app's XZ assets.
package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/ulikunitz/xz"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: build-assets OUTPUT_DIRECTORY")
		os.Exit(1)
	}
	if err := build(os.Args[1]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func build(dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	client := &http.Client{Timeout: 5 * time.Minute}
	for _, asset := range []struct{ name, version string }{
		{"geoip", "20261012"}, {"geosite", "20261110034535"},
	} {
		url := fmt.Sprintf("https://github.com/SagerNet/sing-%s/releases/download/%s/%s.db", asset.name, asset.version, asset.name)
		if err := download(client, url, filepath.Join(dir, asset.name+".db.xz")); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, asset.name+".version.txt"), []byte(asset.version), 0644); err != nil {
			return err
		}
	}
	return nil
}

func download(client *http.Client, url, destination string) error {
	response, err := client.Get(url)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("routing asset download: HTTP %d", response.StatusCode)
	}
	file, err := os.CreateTemp(filepath.Dir(destination), "routing-*.tmp")
	if err != nil {
		return err
	}
	temporary := file.Name()
	defer os.Remove(temporary)
	defer file.Close()
	writer, err := xz.NewWriter(file)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(writer, response.Body)
	closeErr := writer.Close()
	fileErr := file.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if fileErr != nil {
		return fileErr
	}
	return os.Rename(temporary, destination)
}
