// Copyright 2018 The mkcert Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Command mkcert is a simple zero-config tool to make development certificates.
package main

import (
	"bufio"
	"crypto"
	"crypto/x509"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/mail"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"

	"golang.org/x/net/idna"
)

func reconfig_caddyfile(caddyfile_path string) {
	new_tls_config := "tls /etc/certs/localhost.crt /etc/certs/localhost.key"

	caddyfile_content, err := os.Open(caddyfile_path)
	if err != nil {
		log.Fatal("can not find caddyfile from path that you have defined")
	}

	var lines []string
	scanner := bufio.NewScanner(caddyfile_content)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	var new_lines []string
	indent := "    "
	for index, line := range lines {
		if strings.Contains(line, "{") {
			if strings.Contains(lines[index], "\t") {
				indent = "\t"
			}
		} else if strings.Contains(line, "tls") {
			continue
		} else if strings.Contains(line, "}") {
			new_lines = append(new_lines, indent+new_tls_config)
		}

		new_lines = append(new_lines, line)
	}

	output := strings.Join(new_lines[:], "\n")

	d1 := []byte(output)
	ioutil.WriteFile(caddyfile_path, d1, 0644)
}

func reconfig_composefile(service string, compose_path string) {
	pem_config := "- ./localhost.pem:/etc/certs/localhost.crt"
	key_pem_config := "- ./localhost-key.pem:/etc/certs/localhost.key"

	caddyfile_content, err := os.Open(compose_path)
	if err != nil {
		log.Fatal("can not find docker-compose.yml from path that you have defined")
	}

	var lines []string
	scanner := bufio.NewScanner(caddyfile_content)
	for scanner.Scan() {
		text := scanner.Text()
		if strings.Contains(text, pem_config) || strings.Contains(text, key_pem_config) {
			continue
		}

		lines = append(lines, text)
	}

	var new_lines []string
	indent := "    "
	is_service_block := false
	has_volumns := false
	for _, line := range lines {
		if strings.Contains(line, service+":") {
			if strings.Contains(line, "\t") {
				indent = "\t"
			} else if strings.Contains(line, "    ") {
				indent = "    "
			} else if strings.Contains(line, "  ") {
				indent = "  "
			}

			new_lines = append(new_lines, line)
			is_service_block = true
			continue
		}

		if is_service_block {
			if strings.Contains(line, "volumes:") {
				new_lines = append(new_lines, line)
				new_lines = append(new_lines, indent+indent+indent+pem_config)
				new_lines = append(new_lines, indent+indent+indent+key_pem_config)

				has_volumns = true
				continue
			}

			// started new service block
			if strings.Count(line, indent) == 1 {
				if has_volumns {
					new_lines = append(new_lines, indent+indent+"volumes:")
					new_lines = append(new_lines, indent+indent+indent+pem_config)
					new_lines = append(new_lines, indent+indent+indent+key_pem_config)
					new_lines = append(new_lines, "")
				}
				is_service_block = false
			}
		}

		new_lines = append(new_lines, line)
	}

	output := strings.Join(new_lines[:], "\n")

	d1 := []byte(output)
	ioutil.WriteFile(compose_path, d1, 0644)
}

const shortUsage = `Usage of mkcert:

	$ mkcert -install
	Install the local CA in the system trust store.

	$ mkcert example.org
	Generate "example.org.pem" and "example.org-key.pem".

	$ mkcert example.com myapp.dev localhost 127.0.0.1 ::1
	Generate "example.com+4.pem" and "example.com+4-key.pem".

	$ mkcert "*.example.it"
	Generate "_wildcard.example.it.pem" and "_wildcard.example.it-key.pem".

	$ mkcert -uninstall
	Uninstall the local CA (but do not delete it).

`

const advancedUsage = `Advanced options:

	-cert-file FILE, -key-file FILE, -p12-file FILE
	    Customize the output paths.

	-client
	    Generate a certificate for client authentication.

	-ecdsa
	    Generate a certificate with an ECDSA key.

	-pkcs12
	    Generate a ".p12" PKCS #12 file, also know as a ".pfx" file,
	    containing certificate and key for legacy applications.

	-csr CSR
	    Generate a certificate based on the supplied CSR. Conflicts with
	    all other flags and arguments except -install and -cert-file.

	-CAROOT
	    Print the CA certificate and key storage location.

	$CAROOT (environment variable)
	    Set the CA certificate and key storage location. (This allows
	    maintaining multiple local CAs in parallel.)

	$TRUST_STORES (environment variable)
	    A comma-separated list of trust stores to install the local
	    root CA into. Options are: "system", "java" and "nss" (includes
	    Firefox). Autodetected by default.

`

// Version can be set at link time to override debug.BuildInfo.Main.Version,
// which is "(devel)" when building from within the module. See
// golang.org/issue/29814 and golang.org/issue/29228.
var Version string

func main() {
	var (
		installFlag    = flag.Bool("install", false, "")
		uninstallFlag  = flag.Bool("uninstall", false, "")
		pkcs12Flag     = flag.Bool("pkcs12", false, "")
		ecdsaFlag      = flag.Bool("ecdsa", false, "")
		clientFlag     = flag.Bool("client", false, "")
		helpFlag       = flag.Bool("help", false, "")
		carootFlag     = flag.Bool("CAROOT", false, "")
		csrFlag        = flag.String("csr", "", "")
		p12FileFlag    = flag.String("p12-file", "", "")
		versionFlag    = flag.Bool("version", false, "")
		composePathRaw = flag.String("compose", "./docker-compose.yml", "relative path to docker-compose.yml")
		caddyPathRaw   = flag.String("caddy", "./Caddyfile", "relative path to Caddyfile")
		serviceName    = flag.String("service", "https", "https service name in docker-compose.yml")
	)

	flag.Usage = func() {
		fmt.Fprint(flag.CommandLine.Output(), shortUsage)
		fmt.Fprintln(flag.CommandLine.Output(), `For more options, run "mkcert -help".`)
	}
	flag.Parse()

	pwd, err := os.Getwd()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Println(pwd)

	absComposePath := path.Join(pwd, *composePathRaw)

	composeDir := path.Dir(absComposePath)
	certFileFlag := path.Join(composeDir, "localhost.pem")
	keyFileFlag := path.Join(composeDir, "localhost-key.pem")

	composePath := path.Join(pwd, *composePathRaw)
	caddyPath := path.Join(pwd, *caddyPathRaw)

	if *helpFlag {
		fmt.Print(shortUsage)
		fmt.Print(advancedUsage)
		return
	}
	if *versionFlag {
		if Version != "" {
			fmt.Println(Version)
			return
		}
		if buildInfo, ok := debug.ReadBuildInfo(); ok {
			fmt.Println(buildInfo.Main.Version)
			return
		}
		fmt.Println("(unknown)")
		return
	}
	if *carootFlag {
		if *installFlag || *uninstallFlag {
			log.Fatalln("ERROR: you can't set -[un]install and -CAROOT at the same time")
		}
		fmt.Println(getCAROOT())
		return
	}
	if *installFlag && *uninstallFlag {
		log.Fatalln("ERROR: you can't set -install and -uninstall at the same time")
	}
	if *csrFlag != "" && (*pkcs12Flag || *ecdsaFlag || *clientFlag) {
		log.Fatalln("ERROR: can only combine -csr with -install and -cert-file")
	}
	if *csrFlag != "" && flag.NArg() != 0 {
		log.Fatalln("ERROR: can't specify extra arguments when using -csr")
	}
	(&mkcert{
		installMode: *installFlag, uninstallMode: *uninstallFlag, csrPath: *csrFlag,
		pkcs12: *pkcs12Flag, ecdsa: *ecdsaFlag, client: *clientFlag,
		certFile: certFileFlag, keyFile: keyFileFlag, p12File: *p12FileFlag,
	}).Run(flag.Args())

	if !(*installFlag) {
		reconfig_caddyfile(caddyPath)
		reconfig_composefile(*serviceName, composePath)
	}
}

const rootName = "rootCA.pem"
const rootKeyName = "rootCA-key.pem"

type mkcert struct {
	installMode, uninstallMode bool
	pkcs12, ecdsa, client      bool
	keyFile, certFile, p12File string
	csrPath                    string

	CAROOT string
	caCert *x509.Certificate
	caKey  crypto.PrivateKey

	// The system cert pool is only loaded once. After installing the root, checks
	// will keep failing until the next execution. TODO: maybe execve?
	// https://github.com/golang/go/issues/24540 (thanks, myself)
	ignoreCheckFailure bool
}

func (m *mkcert) Run(args []string) {
	m.CAROOT = getCAROOT()
	if m.CAROOT == "" {
		log.Fatalln("ERROR: failed to find the default CA location, set one as the CAROOT env var")
	}
	fatalIfErr(os.MkdirAll(m.CAROOT, 0755), "failed to create the CAROOT")
	m.loadCA()

	if m.installMode {
		m.install()
		if len(args) == 0 {
			return
		}
	} else if m.uninstallMode {
		m.uninstall()
		return
	} else {
		var warning bool
		if storeEnabled("system") && !m.checkPlatform() {
			warning = true
			log.Println("Note: the local CA is not installed in the system trust store.")
		}
		if storeEnabled("nss") && hasNSS && CertutilInstallHelp != "" && !m.checkNSS() {
			warning = true
			log.Printf("Note: the local CA is not installed in the %s trust store.", NSSBrowsers)
		}
		if storeEnabled("java") && hasJava && !m.checkJava() {
			warning = true
			log.Println("Note: the local CA is not installed in the Java trust store.")
		}
		if warning {
			log.Println("Run \"mkcert -install\" for certificates to be trusted automatically ⚠️")
		}
	}

	if m.csrPath != "" {
		m.makeCertFromCSR()
		return
	}

	if len(args) == 0 {
		flag.Usage()
		return
	}

	hostnameRegexp := regexp.MustCompile(`(?i)^(\*\.)?[0-9a-z_-]([0-9a-z._-]*[0-9a-z_-])?$`)
	for i, name := range args {
		if ip := net.ParseIP(name); ip != nil {
			continue
		}
		if email, err := mail.ParseAddress(name); err == nil && email.Address == name {
			continue
		}
		if uriName, err := url.Parse(name); err == nil && uriName.Scheme != "" && uriName.Host != "" {
			continue
		}
		punycode, err := idna.ToASCII(name)
		if err != nil {
			log.Fatalf("ERROR: %q is not a valid hostname, IP, URL or email: %s", name, err)
		}
		args[i] = punycode
		if !hostnameRegexp.MatchString(punycode) {
			log.Fatalf("ERROR: %q is not a valid hostname, IP, URL or email", name)
		}
	}

	m.makeCert(args)
}

func getCAROOT() string {
	if env := os.Getenv("CAROOT"); env != "" {
		return env
	}

	var dir string
	switch {
	case runtime.GOOS == "windows":
		dir = os.Getenv("LocalAppData")
	case os.Getenv("XDG_DATA_HOME") != "":
		dir = os.Getenv("XDG_DATA_HOME")
	case runtime.GOOS == "darwin":
		dir = os.Getenv("HOME")
		if dir == "" {
			return ""
		}
		dir = filepath.Join(dir, "Library", "Application Support")
	default: // Unix
		dir = os.Getenv("HOME")
		if dir == "" {
			return ""
		}
		dir = filepath.Join(dir, ".local", "share")
	}
	return filepath.Join(dir, "mkcert")
}

func (m *mkcert) install() {
	if storeEnabled("system") {
		if m.checkPlatform() {
			log.Print("The local CA is already installed in the system trust store! 👍")
		} else {
			if m.installPlatform() {
				log.Print("The local CA is now installed in the system trust store! ⚡️")
			}
			m.ignoreCheckFailure = true // TODO: replace with a check for a successful install
		}
	}
	if storeEnabled("nss") && hasNSS {
		if m.checkNSS() {
			log.Printf("The local CA is already installed in the %s trust store! 👍", NSSBrowsers)
		} else {
			if hasCertutil && m.installNSS() {
				log.Printf("The local CA is now installed in the %s trust store (requires browser restart)! 🦊", NSSBrowsers)
			} else if CertutilInstallHelp == "" {
				log.Printf(`Note: %s support is not available on your platform. ℹ️`, NSSBrowsers)
			} else if !hasCertutil {
				log.Printf(`Warning: "certutil" is not available, so the CA can't be automatically installed in %s! ⚠️`, NSSBrowsers)
				log.Printf(`Install "certutil" with "%s" and re-run "mkcert -install" 👈`, CertutilInstallHelp)
			}
		}
	}
	if storeEnabled("java") && hasJava {
		if m.checkJava() {
			log.Println("The local CA is already installed in Java's trust store! 👍")
		} else {
			if hasKeytool {
				m.installJava()
				log.Println("The local CA is now installed in Java's trust store! ☕️")
			} else {
				log.Println(`Warning: "keytool" is not available, so the CA can't be automatically installed in Java's trust store! ⚠️`)
			}
		}
	}
	log.Print("")
}

func (m *mkcert) uninstall() {
	if storeEnabled("nss") && hasNSS {
		if hasCertutil {
			m.uninstallNSS()
		} else if CertutilInstallHelp != "" {
			log.Print("")
			log.Printf(`Warning: "certutil" is not available, so the CA can't be automatically uninstalled from %s (if it was ever installed)! ⚠️`, NSSBrowsers)
			log.Printf(`You can install "certutil" with "%s" and re-run "mkcert -uninstall" 👈`, CertutilInstallHelp)
			log.Print("")
		}
	}
	if storeEnabled("java") && hasJava {
		if hasKeytool {
			m.uninstallJava()
		} else {
			log.Print("")
			log.Println(`Warning: "keytool" is not available, so the CA can't be automatically uninstalled from Java's trust store (if it was ever installed)! ⚠️`)
			log.Print("")
		}
	}
	if storeEnabled("system") && m.uninstallPlatform() {
		log.Print("The local CA is now uninstalled from the system trust store(s)! 👋")
		log.Print("")
	} else if storeEnabled("nss") && hasCertutil {
		log.Printf("The local CA is now uninstalled from the %s trust store(s)! 👋", NSSBrowsers)
		log.Print("")
	}
}

func (m *mkcert) checkPlatform() bool {
	if m.ignoreCheckFailure {
		return true
	}

	_, err := m.caCert.Verify(x509.VerifyOptions{})
	return err == nil
}

func storeEnabled(name string) bool {
	stores := os.Getenv("TRUST_STORES")
	if stores == "" {
		return true
	}
	for _, store := range strings.Split(stores, ",") {
		if store == name {
			return true
		}
	}
	return false
}

func fatalIfErr(err error, msg string) {
	if err != nil {
		log.Fatalf("ERROR: %s: %s", msg, err)
	}
}

func fatalIfCmdErr(err error, cmd string, out []byte) {
	if err != nil {
		log.Fatalf("ERROR: failed to execute \"%s\": %s\n\n%s\n", cmd, err, out)
	}
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func binaryExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

var sudoWarningOnce sync.Once

func commandWithSudo(cmd ...string) *exec.Cmd {
	if u, err := user.Current(); err == nil && u.Uid == "0" {
		return exec.Command(cmd[0], cmd[1:]...)
	}
	if !binaryExists("sudo") {
		sudoWarningOnce.Do(func() {
			log.Println(`Warning: "sudo" is not available, and mkcert is not running as root. The (un)install operation might fail. ⚠️`)
		})
		return exec.Command(cmd[0], cmd[1:]...)
	}
	return exec.Command("sudo", append([]string{"--prompt=Sudo password:", "--"}, cmd...)...)
}
