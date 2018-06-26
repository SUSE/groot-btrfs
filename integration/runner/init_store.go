package runner

import (
	"fmt"

	"github.com/SUSE/groot-btrfs/groot"
)

type InitSpec struct {
	UIDMappings    []groot.IDMappingSpec
	GIDMappings    []groot.IDMappingSpec
	StoreSizeBytes int64
}

func (r Runner) InitStore(spec InitSpec) error {
	args := []string{}

	for _, mapping := range spec.UIDMappings {
		args = append(args, "--uid-mapping",
			fmt.Sprintf("%d:%d:%d", mapping.NamespaceID, mapping.HostID, mapping.Size),
		)
	}

	for _, mapping := range spec.GIDMappings {
		args = append(args, "--gid-mapping",
			fmt.Sprintf("%d:%d:%d", mapping.NamespaceID, mapping.HostID, mapping.Size),
		)
	}

	if spec.StoreSizeBytes > 0 {
		args = append(args, "--store-size-bytes", fmt.Sprintf("%d", spec.StoreSizeBytes))
	}

	_, err := r.RunSubcommand("init-store", args...)
	return err
}
