package common

import (
	"fmt"
	"log"
	"path"
	"path/filepath"
)

type CleanPath string

func NewCleanPath(p string) CleanPath {
	absPath, err := filepath.Abs(p)
	if err != nil {
		log.Fatal(err)
	}
	return CleanPath(path.Clean(absPath))
}

type CleanPathSlice []CleanPath

func (c *CleanPathSlice) String() string {
	return fmt.Sprintf("%v", *c)
}

func (c *CleanPathSlice) Set(v string) error {
	*c = append(*c, NewCleanPath(v))
	return nil
}
