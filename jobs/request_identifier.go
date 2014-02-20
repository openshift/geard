package jobs

import (
	"encoding/base64"
	"fmt"
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