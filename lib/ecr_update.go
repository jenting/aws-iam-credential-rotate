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
	"encoding/base64"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	aws "github.com/aws/aws-sdk-go-v2/aws"
	ecr "github.com/aws/aws-sdk-go-v2/service/ecr"
	ecrType "github.com/aws/aws-sdk-go-v2/service/ecr/types"
	"github.com/docker/cli/cli/config/credentials"
)

const (
	awsECRUpdater          = "aws-ecr-updater"
	awsECRUpdaterSecret    = "aws-ecr-updater/secret"
	awsECRUpdaterRegion    = "aws-ecr-updater/region"
	awsECRUpdaterExpiresAt = "aws-ecr-updater/expires-at"
)

// Dont want to have full dependencies on k8s so copy/paste just
// to marshall dockerconfigJson
// https://github.com/kubernetes/kubernetes/blob/master/pkg/credentialprovider/config.go
type DockerConfigJson struct {
	Auths DockerConfig `json:"auths"`
}

// DockerConfig represents the config file used by the docker CLI.
// This config that represents the credentials that should be used
// when pulling images from specific image repositories.
type DockerConfig map[string]DockerConfigEntry

type DockerConfigEntry struct {
	Auth string `json:"auth"`
}

func UpdateECR(client *kubernetes.Clientset, namespace string) {
	secrets, err := getSecretsToUpdate(client, namespace)
	if err != nil {
		log.Fatal(err)
	}

	for _, secret := range secrets.Items {
		log.Infof("Found ECR secret: %s", secret.Name)

		accessKeySecretName := secret.Annotations[awsECRUpdaterSecret]
		region := secret.Annotations[awsECRUpdaterRegion]

		log.Infof("For region: %s", region)

		secret, err := client.CoreV1().Secrets(namespace).Get(context.TODO(), accessKeySecretName, metav1.GetOptions{})
		if err != nil {
			log.Errorf("Unable to get the secret to build AccessKey")
			log.Fatal(err)
		}

		awsConfig, err := NewAWSConfig(region, string(secret.Data[accessKeyIdPropName]), string(secret.Data[secretAccessKeyPropName]), "")
		if err != nil {
			log.Fatal(err)
		}

		// Get an authorization Token from ECR
		svc := ecr.NewFromConfig(awsConfig)

		input := &ecr.GetAuthorizationTokenInput{}
		result, err := svc.GetAuthorizationToken(context.TODO(), input)
		if err != nil {
			log.Errorf("Unable to get an Authorization token from ECR")
			log.Fatal(err)
		}

		log.Infof("Found %d authorizationData", len(result.AuthorizationData))

		err = updateSecretFromToken(client, namespace, secret, result.AuthorizationData[0])
		if err != nil {
			log.Errorf("Unable to update secret with Token")
			log.Fatal(err)
		}
		log.Infof("Secret %q updated with new ECR credentials", secret.Name)
	}
}

// getSecretsToUpdate returns the list of secret that we want to rotate.
func getSecretsToUpdate(client *kubernetes.Clientset, namespace string) (*corev1.SecretList, error) {
	return client.CoreV1().Secrets(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: fmt.Sprintf("%s=true", awsECRUpdater)})
}

// updateSecretFromToken updates a k8s secret with the given AWS ECR AuthorizationData.
func updateSecretFromToken(client *kubernetes.Clientset, namespace string, secret *corev1.Secret, authorizationData ecrType.AuthorizationData) error {
	if secret.Data == nil {
		secret.Data = make(map[string][]byte)
	}
	if secret.Annotations == nil {
		secret.Annotations = make(map[string]string)
	}

	dockerConfigJson := DockerConfigJson{}
	if err := json.Unmarshal(secret.Data[".dockerconfigjson"], &dockerConfigJson); err != nil {
		log.Errorf("Unable to unmarshal .dockerconfigjson")
		return err
	}

	json, err := buildDockerJsonConfig(dockerConfigJson, authorizationData)
	if err != nil {
		log.Errorf("Unable to build dockerJsonConfig from AuthorizationData")
		return err
	}

	secret.Annotations[awsECRUpdaterExpiresAt] = aws.ToTime(authorizationData.ExpiresAt).String()
	secret.Data[".dockerconfigjson"] = json
	_, err = client.CoreV1().Secrets(namespace).Update(context.TODO(), secret, metav1.UpdateOptions{})
	return err
}

func buildDockerJsonConfig(dockerConfigJson DockerConfigJson, authorizationData ecrType.AuthorizationData) ([]byte, error) {
	user := "AWS"
	token := aws.ToString(authorizationData.AuthorizationToken)
	password := decodePassword(token)
	password = password[4:]

	endpoint := credentials.ConvertToHostname(aws.ToString(authorizationData.ProxyEndpoint))
	dockerConfigJson.Auths[endpoint] = DockerConfigEntry{
		Auth: encodeDockerConfigFieldAuth(user, password),
	}
	return json.Marshal(dockerConfigJson)
}

func decodePassword(pass string) string {
	bytes, _ := base64.StdEncoding.DecodeString(pass)
	return string(bytes)
}

func encodeDockerConfigFieldAuth(username, password string) string {
	fieldValue := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(fieldValue))
}
