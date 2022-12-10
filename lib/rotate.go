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
	"bytes"
	"context"
	"fmt"
	"text/template"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	aws "github.com/aws/aws-sdk-go-v2/aws"
	iam "github.com/aws/aws-sdk-go-v2/service/iam"
	iamType "github.com/aws/aws-sdk-go-v2/service/iam/types"
)

const (
	configPropName      = "config"
	credentialsPropName = "credentials"
	rotateKeyLabel      = "aws-rotate-key"
)

func RotateKeys(client *kubernetes.Clientset, namespace string) {
	secrets, err := getSecretsToRotate(client, namespace)
	if err != nil {
		log.Fatal(err)
	}

	for _, secret := range secrets.Items {
		awsConfig, err := NewAWSConfig("", string(secret.Data[accessKeyIdPropName]), string(secret.Data[secretAccessKeyPropName]), "")
		if err != nil {
			log.Fatal(err)
		}

		accessKeyId := string(secret.Data[accessKeyIdPropName])
		oldAccessKeyId := string(secret.Data[accessKeyIdPropName])

		svc := iam.NewFromConfig(awsConfig)

		// List Keys and delete the one(s) we are not using
		keys, err := svc.ListAccessKeys(context.TODO(), nil)
		if err != nil {
			log.Errorf("Unable to use new AccessKey")
			log.Fatal(err)
			continue
		} else {
			for _, k := range keys.AccessKeyMetadata {
				key := aws.ToString(k.AccessKeyId)
				if key != accessKeyId {
					log.Infof("Found orphaned key %s, deleting it", key)
					deleteAccessKey(svc, key)
				}
			}
		}

		// Creating the new AccessKey
		result, err := svc.CreateAccessKey(context.TODO(), nil)
		if err != nil {
			log.Errorf("Unable to create new AccessKey")
			log.Errorf(err.Error())
			continue
		}

		accessKey := result.AccessKey
		log.Infof("Created new AccessKey: %s", aws.ToString(accessKey.AccessKeyId))

		// Wait for the key to be active
		time.Sleep(10 * time.Second)

		// Create a new Session
		newAWSConfig, err := NewAWSConfig("", string(secret.Data[accessKeyIdPropName]), string(secret.Data[secretAccessKeyPropName]), "new")
		if err != nil {
			log.Fatal(err)
		}

		newSvc := iam.NewFromConfig(newAWSConfig)

		// And make sure we can use it
		_, err = newSvc.ListAccessKeys(context.TODO(), nil)
		if err != nil {
			log.Errorf("Unable to use new AccessKey")
			rollbackKeyCreation(svc, accessKey)
			log.Fatal(err)
		}

		// Update the secret in k8s
		err = updateSecret(client, namespace, secret, accessKey)
		if err != nil {
			log.Errorf("Unable to update kubernetes secret")
			rollbackKeyCreation(svc, accessKey)
			log.Fatal(err)
		}

		// Delete the old access key
		err = deleteAccessKey(newSvc, oldAccessKeyId)
		if err != nil {
			log.Errorf("Unable to delete old AccessKey")
			log.Fatal(err)
		} else {
			log.Infof("Successfully deleted old Access key (%s)", oldAccessKeyId)
		}
	}
}

// getSecretsToRotate returns the list of secret that we want to rotate.
func getSecretsToRotate(client *kubernetes.Clientset, namespace string) (*corev1.SecretList, error) {
	return client.CoreV1().Secrets(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: fmt.Sprintf("%s=true", rotateKeyLabel)})
}

// rollbackKeyCreation rolls back the creation of an AccessKey.
func rollbackKeyCreation(iamClient *iam.Client, accessKey *iamType.AccessKey) {
	accessKeyId := aws.ToString(accessKey.AccessKeyId)
	err := deleteAccessKey(iamClient, accessKeyId)
	if err != nil {
		log.Errorf("Unable to delete new AccessKey, there are now probably 2 access keys for this user")
	} else {
		log.Errorf("Rollbacked new AccessKey (%s)", accessKeyId)
	}
}

// AWSCredentials is a struct to store id and secret.
type AWSCredentials struct {
	Profile string
	ID      string
	Secret  string
	Region  string
}

// updateSecret updates a k8s secret with the given AWS AccessKey.
func updateSecret(client *kubernetes.Clientset, namespace string, secret corev1.Secret, accessKey *iamType.AccessKey) error {
	id := aws.ToString(accessKey.AccessKeyId)
	key := aws.ToString(accessKey.SecretAccessKey)

	// Defining template for credentials file
	defaultCredentials := AWSCredentials{"default", id, key, "eu-west-1"}
	openshiftCredentials := AWSCredentials{"openshift", id, key, "eu-west-1"}
	credentialsFileTemplate, err := template.New("credentials").Parse(
		"" +
			"[{{ .Profile}}]\n" +
			"aws_access_key_id={{ .ID}}\n" +
			"aws_secret_access_key={{ .Secret}}\n" +
			"region={{ .Region}}",
	)
	if err != nil {
		return err
	}

	var defaultCredentialsData bytes.Buffer
	var openshiftCredentialsData bytes.Buffer
	credentialsFileTemplate.Execute(&defaultCredentialsData, defaultCredentials)
	credentialsFileTemplate.Execute(&openshiftCredentialsData, openshiftCredentials)

	secret.StringData = make(map[string]string)
	secret.StringData[accessKeyIdPropName] = aws.ToString(accessKey.AccessKeyId)
	secret.StringData[secretAccessKeyPropName] = aws.ToString(accessKey.SecretAccessKey)
	secret.StringData[configPropName] = defaultCredentialsData.String()
	secret.StringData[credentialsPropName] = openshiftCredentialsData.String()

	_, err = client.CoreV1().Secrets(namespace).Update(context.TODO(), &secret, metav1.UpdateOptions{})
	return err
}

// deleteAccessKey deletes an AWS AccessKey based on its Id.
func deleteAccessKey(iamClient *iam.Client, accessKeyId string) error {
	deleteAccessKeyInput := &iam.DeleteAccessKeyInput{
		AccessKeyId: aws.String(accessKeyId),
	}

	_, err := iamClient.DeleteAccessKey(context.TODO(), deleteAccessKeyInput)
	return err
}
