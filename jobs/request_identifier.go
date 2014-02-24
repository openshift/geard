package jobs

import (
	"encoding/base64"
	"fmt"
	"github.com/smarterclayton/geard/config"
	"github.com/smarterclayton/geard/gears"
	"github.com/smarterclayton/geard/utils"
	"path/filepath"
	"strings"
)

type RequestIdentifier []byte

func (r RequestIdentifier) ToShortName() string {
	return strings.Trim(base64.URLEncoding.EncodeToString(r), "=")
}

func (r RequestIdentifier) UnitNameFor() string {
	return fmt.Sprintf("job-%s.service", r.ToShortName())
}

func (r RequestIdentifier) UnitNameForBuild() string {
	return fmt.Sprintf("build-%s.service", r.ToShortName())
}

func (r RequestIdentifier) VersionedUnitPathFor(i gears.Identifier) string {
	return utils.IsolateContentPath(filepath.Join(config.GearBasePath(), "units"), string(i), r.ToShortName())
}
