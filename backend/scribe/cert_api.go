// SPDX-License-Identifier: GPL-3.0-or-later
package scribe

import (
	"errors"
	"fmt"

	"github.com/autogame-17/scribe-studio/backend/scribe/logbus"
	"wx_channel/pkg/certificate"
	"wx_channel/pkg/sphkit"
)

// certName is the Common Name on the bundled SunnyNet CA. Verified via
// `openssl x509 -subject` on backend/core/pkg/certificate/certs/SunnyRoot.cer
// — `subject=... CN=SunnyNet`. We hard-code it here rather than scanning the
// cert at runtime because the bundled cert is fixed and any drift would be a
// build-time error worth catching.
const certName = "SunnyNet"

// CertStatus is the snapshot returned to the dashboard. Installed reports
// whether a cert with CN=SunnyNet is currently trusted; Name is echoed back
// so the UI can show the same string the user would search for in
// Keychain Access (helps debug when Installed reports stale state).
type CertStatus struct {
	Installed bool   `json:"installed"`
	Name      string `json:"name"`
}

// GetCertStatus checks the system trust store for our local CA. Cheap to
// poll — on macOS this shells out to `security find-certificate -a`, which
// is sub-100ms even with thousands of certs. Errors collapse to
// Installed=false rather than bubbling, because "I can't tell" looks the
// same as "not installed" from a UX perspective and we don't want to red-
// banner the dashboard over a transient `security` hiccup.
func (a *App) GetCertStatus() CertStatus {
	ok, err := certificate.CheckHasCertificate(certName)
	if err != nil {
		logbus.Warn("cert", "check: %v", err)
		return CertStatus{Installed: false, Name: certName}
	}
	return CertStatus{Installed: ok, Name: certName}
}

// InstallCert writes the bundled SunnyNet CA into the system trust store.
// On macOS this triggers a SecurityAgent prompt for an admin password —
// `security add-trusted-cert -k /Library/Keychains/System.keychain` requires
// it. The frontend should toast that hint *before* calling this so the user
// doesn't get blindsided by the OS dialog.
func (a *App) InstallCert() error {
	bytes, err := certBytes(a)
	if err != nil {
		return err
	}
	if err := certificate.InstallCertificate(bytes); err != nil {
		logbus.Error("cert", "install: %v", err)
		return err
	}
	logbus.Info("cert", "installed %s", certName)
	return nil
}

// UninstallCert removes the SunnyNet CA from the system trust store. Same
// admin-prompt caveat as Install. Useful for clean uninstall + when the
// cert gets re-issued upstream and the old one needs purging first.
func (a *App) UninstallCert() error {
	if err := certificate.UninstallCertificate(certName); err != nil {
		logbus.Error("cert", "uninstall: %v", err)
		return err
	}
	logbus.Info("cert", "uninstalled %s", certName)
	return nil
}

// certBytes pulls the embedded cert payload out of a sphkit instance. We
// reuse the existing kit if one's already been constructed (during
// resolveDownloadDir or StartProxy) to avoid loading config twice;
// otherwise we spin up a throwaway one. The cert bytes themselves come
// from `//go:embed certs/SunnyRoot.cer` upstream, so they don't depend on
// disk state — but we still go through sphkit.New to keep a single
// codepath for "where do cert files live".
func certBytes(a *App) ([]byte, error) {
	a.mu.Lock()
	kit := a.kit
	a.mu.Unlock()
	if kit == nil {
		k, err := sphkit.New(BuildVersion, BuildMode)
		if err != nil {
			return nil, fmt.Errorf("init kit: %w", err)
		}
		a.mu.Lock()
		a.kit = k
		kit = k
		a.mu.Unlock()
	}
	files := kit.CertFiles()
	if files == nil || len(files.Cert) == 0 {
		return nil, errors.New("bundled CA payload is empty (build without certs?)")
	}
	return files.Cert, nil
}
