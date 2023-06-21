// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You
// may not use this file except in compliance with the License. A copy of
// the License is located at
//
// http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is
// distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific
// language governing permissions and limitations under the License.

package secretcache

const (
	VersionNumber        = "1"
	MajorRevisionNumber  = "1"
	MinorRevisionNumber  = "2"
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
