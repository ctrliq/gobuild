package gobuild

import (
	"fmt"
	"io"

	"github.com/goreleaser/nfpm"
	_ "github.com/goreleaser/nfpm/deb"
	_ "github.com/goreleaser/nfpm/rpm"
)

type Format uint8

const (
	DEB Format = iota
	RPM
)

var formatString = map[Format]string{
	DEB: "deb",
	RPM: "rpm",
}

var formatArch = map[string]map[Format]string{
	"all":     {RPM: "noarch", DEB: "noarch"},
	"amd64":   {RPM: "x86_64", DEB: "amd64"},
	"386":     {RPM: "i386", DEB: "i386"},
	"arm64":   {RPM: "aarch64", DEB: "arm64"},
	"ppc64le": {RPM: "ppc64le", DEB: "ppc64el"},
	"s390x":   {RPM: "s390x", DEB: "s390x"},
	"arm":     {RPM: "armhfp", DEB: "armhf"},
	"arm5":    {RPM: "", DEB: "armel"},
	"arm6":    {RPM: "armhfp", DEB: "armhf"},
	"arm7":    {RPM: "armhfp", DEB: "armhf"},
	"mipsle":  {RPM: "", DEB: "mipsel"},
}

// getPackageInfo returns the target based on suffix and c.
func getPackageInfo(c nfpm.Config, format Format, version string) (*nfpm.Info, error) {
	c.Version = version

	info, err := c.Get(formatString[format])
	if err != nil {
		return nil, fmt.Errorf("package format: %s", err)
	}
	info = nfpm.WithDefaults(info)

	switch format {
	case DEB:
		// Ref: https://www.debian.org/doc/manuals/debian-faq/ch-pkg_basics.en.html#s-pkgname
		info.Target = fmt.Sprintf("%s_%s-%s_%s.%s",
			info.Name,
			info.Version,
			info.Release,
			info.Arch,
			formatString[format])
	case RPM:
		// Ref: http://ftp.rpm.org/max-rpm/ch-rpm-file-format.html
		info.Target = fmt.Sprintf("%s-%s-%s.%s.%s",
			info.Name,
			info.Version,
			info.Release,
			info.Arch,
			formatString[format])
	default:
		return nil, fmt.Errorf("unknown package format: %v", format)
	}

	if err = nfpm.Validate(info); err != nil {
		return nil, err
	}

	return info, nil
}

type Package struct {
	Packager nfpm.Packager
	Info     *nfpm.Info
}

func NewPackage(configReader io.Reader, format Format, version string, arch string) (*Package, error) {
	fmtStr, ok := formatString[format]
	if !ok {
		return nil, fmt.Errorf("unsupported format")
	}

	config, err := nfpm.Parse(configReader)
	if err != nil {
		return nil, fmt.Errorf("while reading configuration: %s", err)
	}

	config.Arch = ""
	if a, ok := formatArch[arch]; ok {
		config.Arch = a[format]
	}
	if config.Arch == "" {
		return nil, fmt.Errorf("unsupported architecture")
	}

	pkg := new(Package)

	pkg.Info, err = getPackageInfo(config, format, version)
	if err != nil {
		return nil, fmt.Errorf("while getting package information: %s", err)
	}
	pkg.Packager, err = nfpm.Get(fmtStr)
	if err != nil {
		return nil, fmt.Errorf("while getting packager: %s", err)
	}

	return pkg, nil
}

func (p *Package) Create(w io.Writer) error {
	if err := p.Packager.Package(p.Info, w); err != nil {
		return fmt.Errorf("while writing package: %s", err)
	}
	return nil
}
