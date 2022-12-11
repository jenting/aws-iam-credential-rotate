/*
Copyright © 2019 Nuxeo

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package lib

import (
	"context"

	aws "github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	awscred "github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/sirupsen/logrus"
)

var log = logrus.New()

const (
	accessKeyIdPropName     = "access_key_id"
	secretAccessKeyPropName = "secret_access_key"
)

func NewAWSConfig(region, accessKeyId, secretAccessKey, session string) (aws.Config, error) {
	return awsconfig.LoadDefaultConfig(
		context.TODO(),
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(
			awscred.NewStaticCredentialsProvider(
				accessKeyId,
				secretAccessKey,
				session,
			),
		),
	)
}
