package nronthego

func InitNROnTheGo(nrAppName, nrLicenceKey string, enableDistributedLogs bool) {
	InitNewRelic(nrAppName, nrLicenceKey)
	if enableDistributedLogs {
		InitLogger(nrAppName, nrLicenceKey)
	}
}
