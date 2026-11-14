package ecslua

import (
	"io"
	"io/fs"
	"os"
)

// source is where a script's text comes from: a file of an fs.FS, or, when
// fsys is nil, the operating system's file system. One type knows the
// difference so System and Runtime.load do not.
type source struct {
	fsys fs.FS // nil: the OS, name is an OS path
	name string
}

// open opens the script for reading.
func (src source) open() (io.ReadCloser, error) {
	if src.fsys == nil {
		return os.Open(src.name)
	}
	return src.fsys.Open(src.name)
}

// stat returns the current version of the script file.
func (src source) stat() (fileVersion, error) {
	var (
		fi  fs.FileInfo
		err error
	)
	if src.fsys == nil {
		fi, err = os.Stat(src.name)
	} else {
		fi, err = fs.Stat(src.fsys, src.name)
	}
	if err != nil {
		return fileVersion{}, err
	}
	return fileVersion{mtime: fi.ModTime(), size: fi.Size()}, nil
}
