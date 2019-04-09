package secretcache

const (
	VersionNumber        = "1"
	MajorRevisionNumber  = "1"
	MinorRevisionNumber  = "0"
	BugfixRevisionNumber = "0"
)

// releaseVersion builds the version string
func releaseVersion() string {
	return VersionNumber + "." + MajorRevisionNumber + "." + MinorRevisionNumber + "." + BugfixRevisionNumber
}

// userAgent builds the user agent string to be appended to outgoing requests to the secrets manager API
func userAgent() string {
	return "AwsSecretCache/" + releaseVersion()
}
