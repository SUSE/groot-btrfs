package driver

import (
	"fmt"
	"strings"

	specs "github.com/opencontainers/runtime-spec/specs-go"

	grootfsgroot "code.cloudfoundry.org/grootfs/groot"
)

// mappingList is an array of specs.LinuxIDMapping
type mappingList []specs.LinuxIDMapping

// NewMappingList returns a list of spec.LinuxIDMappings given a string like "1:65000:2"
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

func (m mappingList) Map(id int) int {
	for _, m := range m {
		if delta := id - int(m.ContainerID); delta < int(m.Size) {
			return int(m.HostID) + delta
		}
	}

	return id
}

func (m mappingList) String() string {
	if len(m) == 0 {
		return "empty"
	}

	var parts []string
	for _, entry := range m {
		parts = append(parts, fmt.Sprintf("%d-%d-%d", entry.ContainerID, entry.HostID, entry.Size))
	}

	return strings.Join(parts, ",")
}

func mappingListToIDMappingSpec(m mappingList) []grootfsgroot.IDMappingSpec {
	result := make([]grootfsgroot.IDMappingSpec, len(m))

	for i, v := range m {
		result[i] = grootfsgroot.IDMappingSpec{
			HostID:      int(v.HostID),
			NamespaceID: int(v.ContainerID),
			Size:        int(v.Size),
		}
	}

	return result
}
