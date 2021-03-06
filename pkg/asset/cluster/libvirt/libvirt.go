// Package libvirt extracts libvirt metadata from install configurations.
package libvirt

import (
	"github.com/metalkube/kni-installer/pkg/types"
	"github.com/metalkube/kni-installer/pkg/types/libvirt"
)

// Metadata converts an install configuration to libvirt metadata.
func Metadata(config *types.InstallConfig) *libvirt.Metadata {
	return &libvirt.Metadata{
		URI: config.Platform.Libvirt.URI,
	}
}
