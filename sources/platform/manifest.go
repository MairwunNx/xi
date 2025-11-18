package platform

import "time"

var (
	appVersion   = "0.0.0"
	appBuildTime = "1970-01-01"
	appStartTime = time.Now()
)

func SetAppManifest(version, buildTime string, startTime time.Time) {
	appVersion = version
	appBuildTime = buildTime
	appStartTime = startTime
}

func GetAppVersion() string {
	return appVersion
}

func GetAppBuildTime() string {
	return appBuildTime
}

func GetAppStartTime() time.Time {
	return appStartTime
}