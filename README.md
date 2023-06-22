## AWS Secrets Manager Go Caching Client

[![Go Reference](https://pkg.go.dev/badge/github.com/aws/aws-secretsmanager-caching-go/secretcache.svg)](https://pkg.go.dev/github.com/aws/aws-secretsmanager-caching-go/secretcache)
[![Tests](https://github.com/aws/aws-secretsmanager-caching-go/actions/workflows/go.yml/badge.svg?event=push)](https://github.com/aws/aws-secretsmanager-caching-go/actions/workflows/go.yml)
[![codecov](https://codecov.io/gh/aws/aws-secretsmanager-caching-go/branch/master/graph/badge.svg?token=JZxWjXaZOC)](https://codecov.io/gh/aws/aws-secretsmanager-caching-go)

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

### Cache Configuration
* `MaxCacheSize int` The maximum number of cached secrets to maintain before evicting secrets that have not been accessed recently.
* `CacheItemTTL int64` The number of nanoseconds that a cached item is considered valid before requiring a refresh of the secret state.  Items that have exceeded this TTL will be refreshed synchronously when requesting the secret value.  If the synchronous refresh failed, the stale secret will be returned.
* `VersionStage string` The version stage that will be used when requesting the secret values for this cache.
* `Hook CacheHook` Used to hook in-memory cache updates.

#### Instantiating Cache with a custom Config and a custom Client
```go

	//Create a custom secretsmanager client
	client := getCustomClient()

	//Create a custom CacheConfig struct
	config := secretcache.CacheConfig{
		MaxCacheSize: secretcache.DefaultMaxCacheSize + 10,
		VersionStage: secretcache.DefaultVersionStage,
		CacheItemTTL: secretcache.DefaultCacheItemTTL,
	}
	
	//Instantiate the cache
	cache, _ := secretcache.New(
		func(c *secretcache.Cache) { c.CacheConfig = config },
		func(c *secretcache.Cache) { c.Client = client },
	)
```

### Getting Help
We use GitHub issues for tracking bugs and caching library feature requests and have limited bandwidth to address them. Please use these community resources for getting help:
* Ask a question on [Stack Overflow](https://stackoverflow.com/) and tag it with [aws-secrets-manager](https://stackoverflow.com/questions/tagged/aws-secrets-manager).
* Open a support ticket with [AWS Support](https://console.aws.amazon.com/support/home#/)
* if it turns out that you may have found a bug, please [open an issue](https://github.com/aws/aws-secretsmanager-caching-python/issues/new).

## License

This library is licensed under the Apache 2.0 License. 
