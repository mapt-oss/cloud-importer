package util

import (
	"bytes"
	"os"
	"path/filepath"
	"text/template"

	"github.com/devtools-qe-incubator/cloud-importer/pkg/util/logging"
)

func WriteTempFile(fileName *string, content string) (*string, error) {
	tmpDir := os.TempDir()
	fullFilePath := filepath.Join(tmpDir, *fileName)
	f, err := os.Create(fullFilePath)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := f.Close(); err != nil {
			logging.Error(err)
		}
	}()
	_, err = f.WriteString(content)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(fullFilePath, 0644); err != nil {
		return nil, err
	}
	return &fullFilePath, err
}

func Template(data any, templateName, templateContent string) (string, error) {
	tmpl, err := template.New(templateName).Parse(templateContent)
	if err != nil {
		return "", err
	}
	buffer := new(bytes.Buffer)
	err = tmpl.Execute(buffer, data)
	if err != nil {
		return "", err
	}
	return buffer.String(), nil
}
