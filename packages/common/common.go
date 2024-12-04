package common

import (
	"fmt"
	"path"
)

type CleanPath string

func NewCleanPath(p string) CleanPath {
	return CleanPath(path.Clean(p))
}

type CleanPathSlice []CleanPath

func (c *CleanPathSlice) String() string {
	return fmt.Sprintf("%v", *c)
}

func (c *CleanPathSlice) Set(v string) error {
	*c = append(*c, NewCleanPath(v))
	return nil
}
