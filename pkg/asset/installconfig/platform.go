package installconfig

import (
	"fmt"
	"sort"

	"github.com/pkg/errors"
	survey "gopkg.in/AlecAivazis/survey.v1"

	"github.com/metalkube/kni-installer/pkg/asset"
	awsconfig "github.com/metalkube/kni-installer/pkg/asset/installconfig/aws"
	baremetalconfig "github.com/metalkube/kni-installer/pkg/asset/installconfig/baremetal"
	libvirtconfig "github.com/metalkube/kni-installer/pkg/asset/installconfig/libvirt"
	openstackconfig "github.com/metalkube/kni-installer/pkg/asset/installconfig/openstack"
	"github.com/metalkube/kni-installer/pkg/types"
	"github.com/metalkube/kni-installer/pkg/types/aws"
	"github.com/metalkube/kni-installer/pkg/types/baremetal"
	"github.com/metalkube/kni-installer/pkg/types/libvirt"
	"github.com/metalkube/kni-installer/pkg/types/none"
	"github.com/metalkube/kni-installer/pkg/types/openstack"
)

// Platform is an asset that queries the user for the platform on which to install
// the cluster.
type platform types.Platform

var _ asset.Asset = (*platform)(nil)

// Dependencies returns no dependencies.
func (a *platform) Dependencies() []asset.Asset {
	return []asset.Asset{}
}

// Generate queries for input from the user.
func (a *platform) Generate(asset.Parents) error {
	platform, err := a.queryUserForPlatform()
	if err != nil {
		return err
	}

	switch platform {
	case aws.Name:
		a.AWS, err = awsconfig.Platform()
		if err != nil {
			return err
		}
	case libvirt.Name:
		a.Libvirt, err = libvirtconfig.Platform()
		if err != nil {
			return err
		}
	case none.Name:
		a.None = &none.Platform{}
	case openstack.Name:
		a.OpenStack, err = openstackconfig.Platform()
		if err != nil {
			return err
		}
	case baremetal.Name:
		a.BareMetal, err = baremetalconfig.Platform()
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown platform type %q", platform)
	}

	return nil
}

// Name returns the human-friendly name of the asset.
func (a *platform) Name() string {
	return "Platform"
}

func (a *platform) queryUserForPlatform() (platform string, err error) {
	err = survey.Ask([]*survey.Question{
		{
			Prompt: &survey.Select{
				Message: "Platform",
				Options: types.PlatformNames,
			},
			Validate: survey.ComposeValidators(survey.Required, func(ans interface{}) error {
				choice := ans.(string)
				i := sort.SearchStrings(types.PlatformNames, choice)
				if i == len(types.PlatformNames) || types.PlatformNames[i] != choice {
					return errors.Errorf("invalid platform %q", choice)
				}
				return nil
			}),
		},
	}, &platform)
	return
}
