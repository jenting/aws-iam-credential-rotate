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
package cmd

import (
	"fmt"
	"os"

	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/nuxeo-cloud/aws-iam-credential-rotate/lib"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var log = logrus.New()

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:   "rotate-iam",
	Short: "A utility that rotates IAM credientials contained in a k8s secret",
	Long:  `A utility that rotates IAM credientials contained in a k8s secret.`,
}

// rotateCmd represents the rotate command.
var rotateCmd = &cobra.Command{
	Use:   "rotate",
	Short: "Rotate the keys labelized.",
	Long:  `Rotates the IAM key`,
	Run: func(cmd *cobra.Command, args []string) {
		kubeConfig, err := ctrl.GetConfig()
		if err != nil {
			log.WithError(err).Fatal("unable to getting Kubernetes client config")
		}

		client, err := kubernetes.NewForConfig(kubeConfig)
		if err != nil {
			log.WithError(err).Fatal("constructing Kubernetes client")
		}

		namespace, _ := os.LookupEnv("NAMESPACE")
		lib.RotateKeys(client, namespace)
	},
}

var ecrUpdate = &cobra.Command{
	Use:   "ecr-update",
	Short: "Update ECR Secret with a new ecr login.",
	Long:  `Update ECR Secret with a new ecr login`,
	Run: func(cmd *cobra.Command, args []string) {
		kubeConfig, err := ctrl.GetConfig()
		if err != nil {
			log.WithError(err).Fatal("unable to getting Kubernetes client config")
		}

		client, err := kubernetes.NewForConfig(kubeConfig)
		if err != nil {
			log.WithError(err).Fatal("constructing Kubernetes client")
		}

		namespace, _ := os.LookupEnv("NAMESPACE")
		lib.UpdateECR(client, namespace)
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.AddCommand(rotateCmd)
	rootCmd.AddCommand(ecrUpdate)
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {}
