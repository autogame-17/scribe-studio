//go:build darwin

package certificate

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

func fetchCertificates() ([]Certificate, error) {
	// Search both the user login keychain and the system keychain. The system
	// keychain is where installCertificate writes (with trust settings); the
	// default search list of a GUI subprocess can omit it, so we list both
	// explicitly to keep "is it installed?" consistent with "where did we
	// install it?".
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, errors.New(fmt.Sprintf("获取用户目录失败，%v\n", err.Error()))
	}
	loginKeychain := home + "/Library/Keychains/login.keychain-db"
	cmd := exec.Command("security", "find-certificate", "-a", loginKeychain, "/Library/Keychains/System.keychain")
	output, err2 := cmd.Output()
	if err2 != nil {
		return nil, errors.New(fmt.Sprintf("获取证书时发生错误，%v\n", err2.Error()))
	}
	var certificates []Certificate
	lines := strings.Split(string(output), "\n")
	for i := 0; i < len(lines)-1; i += 13 {
		if lines[i] == "" {
			continue
		}
		// if i > len(lines)-1 {
		// 	continue
		// }
		cenc := lines[i+5]
		ctyp := lines[i+6]
		hpky := lines[i+7]
		labl := lines[i+9]
		subj := lines[i+12]
		re := regexp.MustCompile(`="([^"]{1,})"`)
		// 找到匹配的字符串
		matches := re.FindStringSubmatch(labl)
		if len(matches) < 1 {
			continue
		}
		label := matches[1]
		certificates = append(certificates, Certificate{
			Thumbprint: "",
			Subject: CertificateSubject{
				CN: label,
				OU: cenc,
				O:  ctyp,
				L:  hpky,
				S:  subj,
				C:  cenc,
			},
		})
	}
	return certificates, nil
}

func installCertificate(cert_data []byte) error {
	cert_file, err := os.CreateTemp("", "SunnyRoot.cer")
	if err != nil {
		return errors.New(fmt.Sprintf("没有创建证书的权限，%v\n", err.Error()))
	}
	defer os.Remove(cert_file.Name())
	if _, err := cert_file.Write(cert_data); err != nil {
		return errors.New(fmt.Sprintf("获取证书失败，%v\n", err.Error()))
	}
	if err := cert_file.Close(); err != nil {
		return errors.New(fmt.Sprintf("生成证书失败，%v\n", err.Error()))
	}
	// Writing to /Library/Keychains/System.keychain requires root. Wrap the
	// command with osascript "with administrator privileges" so macOS prompts
	// the user for their password via the native authorization dialog —
	// otherwise the bare `security` invocation fails with a write-permission
	// error.
	inner := fmt.Sprintf("security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain %s", strconv.Quote(cert_file.Name()))
	script := fmt.Sprintf("do shell script %s with administrator privileges", strconv.Quote(inner))
	ps := exec.Command("osascript", "-e", script)
	output, err2 := ps.CombinedOutput()
	if err2 != nil {
		return errors.New(fmt.Sprintf("安装证书时发生错误，%v\n", string(output)))
	}
	return nil
}

func uninstallCertificate(certificate_name string) error {
	certificates, err := fetchCertificates()
	if err != nil {
		return err
	}
	var matched *Certificate
	for _, cert := range certificates {
		if cert.Subject.CN == certificate_name {
			matched = &cert
			break
		}
	}
	if matched == nil {
		return errors.New("没有找到匹配的根证书")
	}
	cmd := fmt.Sprintf("security delete-certificate -c '%s'", certificate_name)
	ps := exec.Command("bash", "-c", cmd)
	output, err2 := ps.CombinedOutput()
	if err2 != nil {
		return errors.New(fmt.Sprintf("删除证书时发生错误，%v\n", string(output)))
	}
	return nil
}
