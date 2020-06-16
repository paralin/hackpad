package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
)

func main() {
	fmt.Println("go-wasm")
	err := run()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func run() error {
	resp, err := http.Get("/go.zip")
	if err != nil {
		return err
	}

	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	r := bytes.NewReader(buf)

	err = unzip(r, r.Len(), "/go")
	fmt.Println(err)

	dir, err := ioutil.ReadDir("/go")
	if err != nil {
		fmt.Println(err)
		return err
	}

	for _, f := range dir {
		fmt.Println(f.Name())
	}
	info, err := os.Stat("/go")
	if err != nil {
		return err
	}
	fmt.Println("dir perm", info.Mode())
	return makeTestModule()
}

func makeTestModule() error {
	err := ioutil.WriteFile("main.go", []byte(`
package main

func main() {
	println("hello world")
}
`), 0600)
	if err != nil {
		return err
	}

	return ioutil.WriteFile("go.mod", []byte(`
module thing
`), 0600)
}

func unzip(r io.ReaderAt, size int, outPath string) error {
	if err := os.MkdirAll(outPath, 0750); err != nil {
		return err
	}

	z, err := zip.NewReader(r, int64(size))
	if err != nil {
		return err
	}
	return unzipFiles(z.File, outPath)
}

func unzipFiles(files []*zip.File, destDir string) error {
	for _, f := range files {
		filePath, err := validateZipPath(f.Name, destDir)
		if err != nil {
			return err
		}
		if err := unzipFile(f, filePath); err != nil {
			return err
		}
	}
	return nil
}

// validateZipPath prevents "zip slip vulnerability" https://snyk.io/research/zip-slip-vulnerability
func validateZipPath(zipPath string, destDir string) (cleanedPath string, err error) {
	destPrefix := filepath.Clean(destDir) + string(os.PathSeparator)
	filePath := filepath.Join(destPrefix, zipPath)
	if !strings.HasPrefix(filePath, destPrefix) {
		return "", errors.Errorf("%s: illegal zip file path", filePath)
	}
	return filePath, nil
}

func unzipFile(file *zip.File, dest string) error {
	if file.FileInfo().IsDir() {
		return os.Mkdir(dest, file.Mode())
	}
	r, err := file.Open()
	if err != nil {
		return err
	}
	defer r.Close()

	buf, err := ioutil.ReadAll(r)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(dest, buf, file.Mode())
}
