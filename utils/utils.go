//Package utils is helper
package utils

import (
	"encoding/json"
	"io"
	"log"
	"os"

	"github.com/sunweiwe/container/common"
)

func ParseManifest(manifestPath string, mani *common.Manifest) error {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(data, mani); err != nil {
		return err
	}

	return nil
}

func CopyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return nil
}

func DoOrDieWithMessage(err error, msg string) {
	if err != nil {
		log.Fatalf("Fatal error: %s: %v\n", msg, err)
	}
}

func DoOrDie(err error) {
	if err != nil {
		log.Fatalf("Fatal error: %v\n", err)
	}
}

func CreateDirsIfNotExist(dirs []string) error {
	for _, dir := range dirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			if err = os.MkdirAll(dir, 0755); err != nil {
				log.Printf("Error creating directory: %v\n", err)
				return err
			}
		}
	}
	return nil
}

func InitContainerDirs() (err error) {
	dirs := []string{"/var/lib/container", "/var/lib/container/tmp", "/var/lib/container/images", "/var/run/container/containers"}

	return CreateDirsIfNotExist(dirs)
}

func CreateDirsIfDontExist(dirs []string) error {
	for _, dir := range dirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			if err = os.MkdirAll(dir, 0755); err != nil {
				log.Printf("Error creating directory: %v\n", err)
				return err
			}
		}
	}
	return nil
}
