package util

import (
	"path/filepath"
	"statemachine/constants"
)

func PathDir(root string, specifcDirName string) string {
	return filepath.Join(root, specifcDirName)
}

func DBDir(root string) string  { return filepath.Join(root, constants.DbDirName) }
func LogDir(root string) string { return filepath.Join(root, constants.LogDirName) }
