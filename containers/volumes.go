package containers

import (
	"errors"
	"fmt"
	"strings"
)

type Volume struct {
	Path string
}

func (v Volume) String() string {
	return v.Path
}

type VolumeMount struct {
	Path     string
	HostPath string
	ReadOnly bool
}

var invalidVolumeMount = VolumeMount{Path: "invalid"}

func NewVolumeMountFromString(s string) (VolumeMount, error) {
	var (
		fields   []string
		path     string
		hostPath string
		readOnly bool
		size     int
	)

	fields = strings.Split(s, ":")
	size = len(fields)
	if size < 2 || size > 3 {
		return invalidVolumeMount, errors.New("Invalid volume-mount spec: " + s)
	}

	path = fields[0]
	hostPath = fields[1]

	if size == 3 {
		mode := fields[2]
		switch mode {
		case "ro":
			readOnly = true
			break
		case "rw":
			break
		default:
			return invalidVolumeMount, errors.New("Invalid volume-mount spec")
		}
	}

	return VolumeMount{path, hostPath, readOnly}, nil
}

func (v VolumeMount) String() string {
	spec := fmt.Sprintf("%s:%s", v.Path, v.HostPath)

	if v.ReadOnly {
		spec += ":ro"
	}

	return spec
}

type VolumeConfig struct {
	Volumes      []Volume
	VolumeMounts []VolumeMount
}

func VolumeConfigFromString(s string) (*VolumeConfig, error) {
	vc := new(VolumeConfig)
	specs := strings.Split(s, ",")

	for _, spec := range specs {
		if strings.Index(spec, ":") == -1 {
			vc.Volumes = append(vc.Volumes, Volume{Path: spec})
		} else {
			vm, err := NewVolumeMountFromString(spec)
			if err != nil {
				return nil, err
			}

			vc.VolumeMounts = append(vc.VolumeMounts, vm)
		}
	}

	return vc, nil
}

func (vc VolumeConfig) String() string {
	return vc.ToVolumeSpec() + " " + vc.ToBindMountSpec()
}

func (vc VolumeConfig) ToVolumeSpec() string {
	str := ""

	if len(vc.Volumes) != 0 {
		for _, v := range vc.Volumes {
			str += " -v " + v.String()
		}
	}

	return str
}

func (vc VolumeConfig) ToBindMountSpec() string {
	str := ""

	if len(vc.VolumeMounts) != 0 {
		for _, v := range vc.VolumeMounts {
			str += " -v " + v.String()
		}
	}

	return str
}
