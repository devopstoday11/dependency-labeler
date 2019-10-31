package rootfs

import (
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	v1 "github.com/google/go-containerregistry/pkg/v1"

	"github.com/google/go-containerregistry/pkg/v1/mutate"

	"github.com/docker/docker/pkg/archive"

	"github.com/pkg/errors"
)

type Interface interface {
	GetFileContent(path string) (string, error)
	GetDirContents(path string) ([]string, error)
}

type RootFS struct {
	rootfsLocation string
}

func (rfs *RootFS) GetDirContents(path string) ([]string, error) {
	var fileContents []string
	files, err := ioutil.ReadDir(filepath.Join(rfs.rootfsLocation, path))
	if err != nil {
		return fileContents, errors.Wrapf(err, "could not find directory in rootfs: %s", err)
	}

	for _, f := range files {
		if f.IsDir() {
			continue
		}

		thisFileContent, err := rfs.GetFileContent(filepath.Join(path, f.Name()))
		if err != nil {
			return fileContents, errors.Wrapf(err, "could not find file in directory in rootfs: %s", err)
		}
		fileContents = append(fileContents, thisFileContent)
	}

	return fileContents, nil
}

func (rfs *RootFS) GetFileContent(path string) (string, error) {
	fileBytes, err := ioutil.ReadFile(filepath.Join(rfs.rootfsLocation, path))
	if err != nil {
		return "", errors.Wrapf(err, "could not find file in rootfs: %s", err)
	}
	return string(fileBytes), nil
}

func New(image v1.Image) (RootFS, error) {
	var rootfs = ""
	var err error

	f, err := ioutil.TempFile("", "image")
	if err != nil {
		return RootFS{}, errors.Wrap(err, "Could not create temp file.")
	}

	fs := mutate.Extract(image)

	rootfs, err = ioutil.TempDir("", "deplab-rootfs-")
	if err != nil {
		return RootFS{}, errors.Wrap(err, "Could not create rootfs temp directory.")
	}

	err = archive.Untar(fs, rootfs, &archive.TarOptions{NoLchown: true})
	if err != nil {
		return RootFS{}, errors.Wrapf(err, "Could not untar from tar %s to temp directory %s.", f.Name(), rootfs)
	}

	return RootFS{rootfsLocation: rootfs}, nil
}

func (rfs *RootFS) Cleanup() {
	err := os.RemoveAll(rfs.rootfsLocation)
	if err != nil {
		log.Printf("could not clean up rootfs location: %s. %s\n", rfs.rootfsLocation, err)
	}
}
