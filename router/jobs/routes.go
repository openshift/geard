package jobs

import (
	"errors"
	"fmt"
	"github.com/openshift/geard/router"
)

type FrontendDescription struct {
	router.Frontend
	CertificateId router.Identifier `json:"CertificateId,omitempty"`
	BackendId     router.Identifier `json:"BackendId,omitempty"`
}

type BackendDescription struct {
	router.Backend
}

func (f *FrontendDescription) CheckCertificate(certs router.Certificates) error {
	if f.CertificateId == "" {
		return nil
	}
	for i := range certs {
		if f.CertificateId == certs[i].Id {
			f.Certificate = &certs[i]
			f.CertificateId = ""
			return nil
		}
	}
	return errors.New(fmt.Sprintf("No certificate with id %s is defined", f.CertificateId))
}
