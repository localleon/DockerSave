package main

import (
	"archive/tar"
	"io"
	"os"
	"path/filepath"
	"strings"
)

//Tar creates an .tar archive out of its input source directory. Used for creating the final image file out of the multiple layers
func Tar(source, target string) error {
	tarfile, err := os.Create(target)
	if err != nil {
		return err
	}
	defer tarfile.Close()

	tarball := tar.NewWriter(tarfile)
	defer tarball.Close()

	baseDir := "./" // set top level directory to relativ path so we dont need a sub-directory

	return filepath.Walk(source,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			header, err := tar.FileInfoHeader(info, info.Name())
			if err != nil {
				return err
			}

			// Rewrite Header so we have a relative path that doesn't include ./golayer/S
			header.Name = filepath.Join(baseDir, strings.TrimPrefix(path, source))

			if err := tarball.WriteHeader(header); err != nil {
				return err
			}

			if info.IsDir() {
				return nil
			}

			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()
			_, err = io.Copy(tarball, file)
			return err
		})
}
