module github.com/nuxeo-cloud/aws-iam-credential-rotate

go 1.12

require (
	github.com/aws/aws-sdk-go-v2 v1.17.2
	github.com/aws/aws-sdk-go-v2/config v1.18.4
	github.com/aws/aws-sdk-go-v2/credentials v1.13.4
	github.com/aws/aws-sdk-go-v2/service/ecr v1.17.24
	github.com/aws/aws-sdk-go-v2/service/iam v1.18.24
	github.com/docker/cli v20.10.21+incompatible
	github.com/docker/docker-credential-helpers v0.7.0 // indirect
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/cobra v1.4.0
	gotest.tools/v3 v3.4.0 // indirect
	k8s.io/api v0.26.0
	k8s.io/apimachinery v0.26.0
	k8s.io/client-go v0.25.0
	sigs.k8s.io/controller-runtime v0.13.1
)
