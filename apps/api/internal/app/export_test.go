package app

import "time"

func SetVersionTimeout(d time.Duration) func() {
	old := versionTimeout
	versionTimeout = d
	return func() { versionTimeout = old }
}
