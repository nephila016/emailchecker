package main

import (
	"github.com/yourusername/emailverify/cmd"
)

var (
	Version   = "dev"
	BuildTime = "unknown"
)

func main() {
	cmd.SetVersionInfo(Version, BuildTime)
	cmd.Execute()
}
