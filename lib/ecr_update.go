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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/docker/cli/cli/config/credentials"
	"github.com/ericchiang/k8s"
	corev1 "github.com/ericchiang/k8s/apis/core/v1"
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

func UpdateECR(client *k8s.Client, namespace string) {
	secrets, err := getSecretsToUpdate(client, namespace)
	if err != nil {
		log.Fatal(err)
	}

	for _, secret := range secrets.Items {
		log.Infof("Found ECR secret: %s", *secret.Metadata.Name)

		accessKeySecretName := secret.Metadata.Annotations["aws-ecr-updater/secret"]
		region := secret.Metadata.Annotations["aws-ecr-updater/region"]

		log.Infof("For region: %s", region)

		var accessKeySecret corev1.Secret
		if err := client.Get(context.TODO(), namespace, accessKeySecretName, &accessKeySecret); err != nil {
			log.Errorf("Unable to get the secret to build AccessKey")
			log.Fatal(err)
		}

		mySession := createSessionFromSecret(&accessKeySecret)

		// Get an authorization Token from ECR
		svc := ecr.New(mySession, aws.NewConfig().WithRegion(region))

		input := &ecr.GetAuthorizationTokenInput{}

		result, err := svc.GetAuthorizationToken(input)
		if err != nil {
			log.Errorf("Unable to get an Authorization token from ECR")
			log.Fatal(err)
		}

		log.Infof("Found %d authorizationData", len(result.AuthorizationData))

		err = updateSecretFromToken(client, secret, result.AuthorizationData[0])
		if err != nil {
			log.Errorf("Unable to update secret with Token")
			log.Fatal(err)
		}
		log.Infof("Secret %q updated with new ECR credentials", *secret.Metadata.Name)
	}

}

// getSecretsToUpdate returns the list of secret that we want to rotate.
func getSecretsToUpdate(client *k8s.Client, namespace string) (*corev1.SecretList, error) {
	l := new(k8s.LabelSelector)
	l.Eq("aws-ecr-updater", "true")

	var secrets corev1.SecretList
	if err := client.List(context.TODO(), namespace, &secrets, l.Selector()); err != nil {
		return nil, err
	}
	return &secrets, nil
}

// updateSecretFromToken updates a k8s secret with the given AWS ECR AuthorizationData.
func updateSecretFromToken(client *k8s.Client, secret *corev1.Secret, authorizationData *ecr.AuthorizationData) error {
	if secret.Data == nil {
		secret.Data = make(map[string][]byte)
	}
	if secret.Metadata.Annotations == nil {
		secret.Metadata.Annotations = make(map[string]string)
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

	secret.Metadata.Annotations["aws-ecr-updater/expires-at"] = aws.TimeValue(authorizationData.ExpiresAt).String()
	secret.Data[".dockerconfigjson"] = json
	return client.Update(context.TODO(), secret)
}

func buildDockerJsonConfig(dockerConfigJson DockerConfigJson, authorizationData *ecr.AuthorizationData) ([]byte, error) {
	user := "AWS"
	token := aws.StringValue(authorizationData.AuthorizationToken)
	password := decodePassword(token)
	password = password[4:]

	endpoint := credentials.ConvertToHostname(aws.StringValue(authorizationData.ProxyEndpoint))
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
