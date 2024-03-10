package token

import (
	"os"

	"golang.org/x/oauth2"
)

type FileTokenSource struct {
	path string
}

func NewFileTokenSource(path string) *FileTokenSource {
	return &FileTokenSource{path: path}
}

func (f *FileTokenSource) Token() (*oauth2.Token, error) {
	b, err := os.ReadFile(f.path)
	if err != nil {
		return nil, err
	}
	return &oauth2.Token{
		AccessToken: string(b),
	}, nil
}
