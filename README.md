## AWS Secrets Manager Go Caching Client

The AWS Secrets Manager Go caching client enables in-process caching of secrets for Go applications.

## Getting Started

### Required Prerequisites
To use this client you must have:

* **A Go development environment**

  If you do not have one, go to [Golang Getting Started](https://golang.org/doc/install) on The Go Programming Language website, then download and install Go.

An Amazon Web Services (AWS) account to access secrets stored in AWS Secrets Manager and use AWS SDK for Go.

* **To create an AWS account**, go to [Sign In or Create an AWS Account](https://portal.aws.amazon.com/gp/aws/developer/registration/index.html) and then choose **I am a new user.** Follow the instructions to create an AWS account.

* **To create a secret in AWS Secrets Manager**, go to [Creating Secrets](https://docs.aws.amazon.com/secretsmanager/latest/userguide/manage_create-basic-secret.html) and follow the instructions on that page.


### Get Started

The following code sample demonstrates how to get started:

1. Instantiate the caching client.
2. Request secret.

```go
// This example shows how an AWS Lambda function can be written
// to retrieve a cached secret from AWS Secrets Manager caching
// client.
package main

import (
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-secretsmanager-caching-go/secretcache"
)

var(
	secretCache, _ = secretcache.New()
)

func HandleRequest(secretId string) string {
	result, _ := secretCache.GetSecretString(secretId)
	// Use secret to connect to secured resource.
	return "Success"
}

func main() {
	lambda.Start(HandleRequest)
}
```

## License

This library is licensed under the Apache 2.0 License. 