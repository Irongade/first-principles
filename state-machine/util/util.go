package util

import (
	"fmt"
	"path/filepath"
	"statemachine/constants"
	"strings"
)

func PathDir(root string, specifcDirName string) string {
	return filepath.Join(root, specifcDirName)
}

func DBDir(root string) string  { return filepath.Join(root, constants.DbDirName) }
func LogDir(root string) string { return filepath.Join(root, constants.LogDirName) }

func ParsePeers(raw string, self string) (map[string]string, error) {
	peers := make(map[string]string)

	found := false

	for _, item := range strings.Split(raw, ",") {
		id, addr, ok := strings.Cut(strings.TrimSpace(item), "=")

		if !ok || id == "" || addr == "" {
			return nil, fmt.Errorf("Bad peers value from env variable: %s, raw valueL %s", item, raw)
		}

		if id == self {
			found = true
			continue
		}

		peers[id] = addr
	}

	if !found {
		return nil, fmt.Errorf("Could not find this node in NODE_PEERS env variable. self id: %s, raw value: %s", self, raw)
	}

	return peers, nil
}
