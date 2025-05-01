package nronthego

import "github.com/xlogn/nronthego/pkg"

func InitNROnTheGo(nrAppName, nrLicenceKey string, enableDistributedLogs bool) {
	pkg.InitNewRelic(nrAppName, nrLicenceKey)
	if enableDistributedLogs {
		pkg.InitLogger(nrAppName, nrLicenceKey)
	}
}
