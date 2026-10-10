package buildinfo

import (
	"runtime"
	"runtime/debug"
	"strings"
)

const desktopKitModule = "github.com/wanstu/wails-desktop-kit"

var (
	Version   = "dev"
	Commit    = ""
	BuildTime = ""
)

type Info struct {
	Version      string `json:"version"`
	Commit       string `json:"commit,omitempty"`
	BuildTime    string `json:"build_time,omitempty"`
	Modified     bool   `json:"modified,omitempty"`
	Platform     string `json:"platform"`
	Architecture string `json:"architecture"`
	GoVersion    string `json:"go_version"`
	KitVersion   string `json:"kit_version,omitempty"`
}

func Current() Info {
	info := Info{
		Version:      strings.TrimSpace(Version),
		Commit:       strings.TrimSpace(Commit),
		BuildTime:    strings.TrimSpace(BuildTime),
		Platform:     runtime.GOOS,
		Architecture: runtime.GOARCH,
		GoVersion:    runtime.Version(),
	}
	if info.Version == "" {
		info.Version = "dev"
	}
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range bi.Settings {
			switch setting.Key {
			case "vcs.revision":
				if info.Commit == "" {
					info.Commit = setting.Value
				}
			case "vcs.modified":
				info.Modified = setting.Value == "true"
			}
		}
		for _, dep := range bi.Deps {
			if dep.Path != desktopKitModule {
				continue
			}
			version := dep.Version
			if dep.Replace != nil && strings.TrimSpace(dep.Replace.Version) != "" {
				version = dep.Replace.Version
			}
			info.KitVersion = strings.TrimSpace(version)
			break
		}
	}
	if len(info.Commit) > 12 {
		info.Commit = info.Commit[:12]
	}
	return info
}
