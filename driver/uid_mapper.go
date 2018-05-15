package driver

import (
	"fmt"
	"strings"

	specs "github.com/opencontainers/runtime-spec/specs-go"
)

type MappingList []specs.LinuxIDMapping

// Given a string like "1:65000:2" this function returns a list of
// spec.LinuxIDMappings
func NewMappingList(mappings []string) ([]specs.LinuxIDMapping, error) {
	result := []specs.LinuxIDMapping{}

	for _, v := range mappings {
		mapping := specs.LinuxIDMapping{}
		_, err := fmt.Sscanf(v, "%d:%d:%d", &mapping.ContainerID, &mapping.HostID, &mapping.Size)
		if err != nil {
			return nil, err
		}
		result = append(result, mapping)
	}

	return result, nil
}

func (m MappingList) Map(id int) int {
	for _, m := range m {
		if delta := id - int(m.ContainerID); delta < int(m.Size) {
			return int(m.HostID) + delta
		}
	}

	return id
}

func (m MappingList) String() string {
	if len(m) == 0 {
		return "empty"
	}

	var parts []string
	for _, entry := range m {
		parts = append(parts, fmt.Sprintf("%d-%d-%d", entry.ContainerID, entry.HostID, entry.Size))
	}

	return strings.Join(parts, ",")
}
