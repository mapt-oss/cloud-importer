package util

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/devtools-qe-incubator/cloud-importer/pkg/util/logging"
)

func WriteTempFile(content string) (*string, error) {
	tmpFile, err := os.CreateTemp("", fmt.Sprintf("%s-", filepath.Base(os.Args[0])))
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := tmpFile.Close(); err != nil {
			logging.Error(err)
		}
	}()
	_, err = tmpFile.WriteString(content)
	fileName := tmpFile.Name()
	if err := os.Chmod(fileName, 0644); err != nil {
		panic(err)
	}
	return &fileName, err
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
