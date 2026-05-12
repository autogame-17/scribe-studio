package scribe

// VersionInfo surfaces the build identity to the About screen.
type VersionInfo struct {
	App       string `json:"app"`
	Core      string `json:"core"`
	Commit    string `json:"commit"`
	BuildDate string `json:"buildDate"`
}

// These are injected at build time via -ldflags "-X ..." in the release
// workflow; zero values are fine for dev.
var (
	BuildVersion = "0.0.0-dev"
	BuildMode    = "debug"
	BuildCoreRev = "unknown"
	BuildCommit  = "unknown"
	BuildDate    = "unknown"
)

func (a *App) GetVersion() VersionInfo {
	return VersionInfo{
		App:       BuildVersion,
		Core:      BuildCoreRev,
		Commit:    BuildCommit,
		BuildDate: BuildDate,
	}
}
