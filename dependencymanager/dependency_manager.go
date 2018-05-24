package dependencymanager

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	errorspkg "github.com/pkg/errors"
)

// DependencyManager knows which volumes are in use by which images.
// It's used so we can tell what can and can't be removed.
type DependencyManager struct {
	dependenciesPath string
}

// NewDependencyManager creates a new DependencyManager
func NewDependencyManager(dependenciesPath string) *DependencyManager {
	return &DependencyManager{
		dependenciesPath: dependenciesPath,
	}
}

// Register creates a JSON file that contains all volumes in use by an image.
func (d *DependencyManager) Register(id string, chainIDs []string) error {
	data, err := json.Marshal(chainIDs)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(d.filePath(id), data, 0666)
}

// Deregister deletes the file created by Register
func (d *DependencyManager) Deregister(id string) error {
	return os.Remove(d.filePath(id))
}

// Dependencies returns the dependencies stored in the file created by Register
func (d *DependencyManager) Dependencies(id string) ([]string, error) {
	f, err := os.Open(d.filePath(id))
	if err != nil && os.IsNotExist(err) {
		return nil, errorspkg.Errorf("image `%s` not found", id)
	}
	if err != nil {
		return nil, err
	}

	var chainIDs []string
	if err := json.NewDecoder(f).Decode(&chainIDs); err != nil {
		return nil, err
	}

	return chainIDs, nil
}

func (d *DependencyManager) filePath(id string) string {
	escapedID := strings.Replace(id, "/", "__", -1)
	return filepath.Join(d.dependenciesPath, fmt.Sprintf("%s.json", escapedID))
}
