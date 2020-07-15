package gobuild

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type ArchiveFormat uint8

const (
	TgzArchive ArchiveFormat = iota
	ZipArchive
)

type GitArchive struct {
	gd     *GitDescription
	prefix string
}

func NewGitArchive(prefix string) (*GitArchive, error) {
	var err error
	ga := new(GitArchive)

	ga.gd, err = GitDescribe()
	if err != nil {
		return nil, err
	}

	if ga.gd.tag == nil {
		return nil, fmt.Errorf("no tag found to create archive from")
	} else if ga.gd.n > 0 {
		tagName := ga.gd.tag.Name
		return nil, fmt.Errorf("tag %s must also be HEAD", tagName)
	}

	ga.prefix = prefix
	return ga, nil
}

func (ga *GitArchive) Create(format ArchiveFormat, w io.Writer, extraFiles ...string) error {
	switch format {
	case TgzArchive:
		return ga.createTgzArchive(w, extraFiles...)
	case ZipArchive:
		return ga.createZipArchive(w, extraFiles...)
	}

	return nil
}

func (ga *GitArchive) createTgzArchive(w io.Writer, extraFiles ...string) error {
	gzipWriter := gzip.NewWriter(w)
	defer gzipWriter.Close()

	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	for _, entry := range append(ga.gd.ListEntries(), extraFiles...) {
		err := addEntryToTar(ga.prefix, entry, tarWriter)
		if err != nil {
			return fmt.Errorf("while adding file %s to tar archive: %s", entry, err)
		}
	}

	return nil
}

func (ga *GitArchive) createZipArchive(w io.Writer, extraFiles ...string) error {
	zipWriter := zip.NewWriter(w)

	for _, entry := range append(ga.gd.ListEntries(), extraFiles...) {
		err := addEntryToZip(ga.prefix, entry, zipWriter)
		if err != nil {
			return fmt.Errorf("while adding file %s to zip archive: %s", entry, err)
		}
	}

	return zipWriter.Close()
}

func addEntryToTar(prefix, path string, w *tar.Writer) error {
	fi, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("while getting information for file %s: %s", path, err)
	}

	link := ""
	if fi.Mode()&os.ModeSymlink != 0 {
		link, err = os.Readlink(path)
		if err != nil {
			return fmt.Errorf("while reading symlink %s: %s", path, err)
		}
	}

	header, err := tar.FileInfoHeader(fi, link)
	if err != nil {
		return fmt.Errorf("while getting tar header for file %s: %s", path, err)
	}
	header.Name = filepath.Join(prefix, path)
	if fi.IsDir() {
		header.Name += "/"
	}

	err = w.WriteHeader(header)
	if err != nil {
		return fmt.Errorf("while writing tar header for file %s: %s", path, err)
	}

	if fi.Mode().IsRegular() {
		file, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("while opening file %s: %s", path, err)
		}
		defer file.Close()

		_, err = io.Copy(w, file)
		if err != nil {
			return fmt.Errorf("while copying file %s to tar: %s", path, err)
		}
	}

	return nil
}

func addEntryToZip(prefix, path string, w *zip.Writer) error {
	fi, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("while getting information for file %s: %s", path, err)
	}

	header, err := zip.FileInfoHeader(fi)
	if err != nil {
		return fmt.Errorf("while getting zip information header for file %s: %s", path, err)
	}
	header.Name = filepath.Join(prefix, path)
	header.Method = zip.Deflate

	if fi.Mode().IsDir() {
		header.Name += "/"
	}

	f, err := w.CreateHeader(header)
	if err != nil {
		return fmt.Errorf("while create zip file %s: %s", path, err)
	}

	if fi.Mode().IsRegular() {
		file, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("while opening file %s: %s", path, err)
		}
		defer file.Close()

		_, err = io.Copy(f, file)
		if err != nil {
			return fmt.Errorf("while copying file %s to zip archive: %s", path, err)
		}
	} else {
		var data []byte

		if fi.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err != nil {
				return fmt.Errorf("while reading symlink %s: %s", path, err)
			}
			data = []byte(target)
		}

		_, err = f.Write(data)
		if err != nil {
			return fmt.Errorf("while copying file %s to zip archive: %s", path, err)
		}
	}

	return nil
}
