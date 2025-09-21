package version

import (
	"fmt"
	"runtime"
)

var (
	GoVersion  = runtime.Version()
	CommitId   string
	BranchName string
	BuildTime  string
	AppVersion string
)

func PrintVersion() string {
	return fmt.Sprintf("go version: %s\r\n", GoVersion) + fmt.Sprintf("git commit ID: %s\r\n", CommitId) + fmt.Sprintf("git branch name: %s\r\n", BranchName) + fmt.Sprintf("app build time: %s\r\n", BuildTime) + fmt.Sprintf("app version: %s\r\n", AppVersion)
}
