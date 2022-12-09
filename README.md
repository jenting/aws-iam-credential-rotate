AWS IAM/ECR Credential Rotation Tool
=======================

This is a really simple tool that takes a secret containing IAM credentials to
- Rotate IAM credentials.
- Rotate ECR credentials.


How to use
==========


Rotate IAM Credential
--------------

### Rotate IAM credential policy

The following policy has to be created and attached to the user so that he can change his own keys:

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "VisualEditor0",
            "Effect": "Allow",
            "Action": [
                "iam:UpdateAccessKey",
                "iam:CreateAccessKey",
                "iam:ListAccessKeys",
                "iam:DeleteAccessKey"
            ],
            "Resource": "arn:aws:iam::*:user/${aws:username}"
        }
    ]
}
```

### IAM user

Create a user in AWS and attach the previous policy to it. Generate an access key for the user.

### Secret in Kubernetes

The following secret will hold the initial credentials of the user.

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: aws-iam-user-credentials
  labels:
    aws-rotate-key: "true"
stringData:
  access_key_id: AKIASX3NJFVAYLY464VG
  secret_access_key: xxxxxxxxxxxxxxxxxxxxxxxxx
```

### CronJob

Finally we need a CRON job that runs with privileges to list secrets:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: aws-credentials-updater

---

kind: Role
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: secret-edit
rules:
- apiGroups: [""]
  resources: ["secrets"]
  verbs: ["get", "watch", "list", "update"]
---
kind: RoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: aws-credentials-updater-rolebinding
subjects:
- kind: ServiceAccount
  name: aws-credentials-updater
roleRef:
  kind: Role
  name: secret-edit
  apiGroup: rbac.authorization.k8s.io

---

apiVersion: batch/v1
kind: CronJob
metadata:
  name: rotate-keys
spec:
  jobTemplate:
    spec:
      backoffLimit: 10
      template:
        spec:
          containers:
          - name: rotate-keys
            image: jenting/aws-iam-credential-rotate
          restartPolicy: OnFailure
          securityContext:
            allowPrivilegeEscalation: false
          serviceAccount: aws-credentials-updater
          serviceAccountName: aws-credentials-updater
  concurrencyPolicy: Replace
  schedule: "* */6 * * *"
  successfulJobsHistoryLimit: 1
  failedJobsHistoryLimit: 10
```

Rotate ECR Credentials
----------------------

### Create a secret

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: aws-ecr-credentials-us-west-1
  labels:
    aws-ecr-updater: "true"
  annotations:
    aws-ecr-updater/secret: "aws-iam-user-credentials"
    aws-ecr-updater/region: "us-west-1"
type: kubernetes.io/dockerconfigjson
stringData:
  .dockerconfigjson: "{}"
```

### Attach a policy to the IAM user

Create the following policy and attach it to the IAM user:

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "VisualEditor0",
            "Effect": "Allow",
            "Action": [
                "ecr:GetAuthorizationToken",
                "ecr:BatchCheckLayerAvailability",
                "ecr:GetDownloadUrlForLayer",
                "ecr:GetRepositoryPolicy",
                "ecr:DescribeRepositories",
                "ecr:ListImages",
                "ecr:DescribeImages",
                "ecr:BatchGetImage",
                "ecr:InitiateLayerUpload",
                "ecr:UploadLayerPart",
                "ecr:CompleteLayerUpload",
                "ecr:PutImage"
            ],
            "Resource": "*"
        }
    ]
}
```

### CronJob

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: rotate-ecr
spec:
  jobTemplate:
    spec:
      backoffLimit: 10
      template:
        spec:
          containers:
          - command:
            - /aws-iam-credential-rotate
            - ecr-update
            image: jenting/aws-iam-credential-rotate
            name: rotate-ecr
          securityContext:
            allowPrivilegeEscalation: false
          serviceAccount: aws-credentials-updater
          serviceAccountName: aws-credentials-updater
  concurrencyPolicy: Replace
  schedule: "* */6 * * *"
  successfulJobsHistoryLimit: 1
  failedJobsHistoryLimit: 10
```

# Licensing

Most of the source code in the Nuxeo Platform is copyright Nuxeo and
contributors, and licensed under the Apache License, Version 2.0.

See the [LICENSE](LICENSE) file and the documentation page [Licenses](http://doc.nuxeo.com/x/gIK7) for details.

# About Nuxeo

Nuxeo dramatically improves how content-based applications are built, managed and deployed, making customers more agile, innovative and successful. Nuxeo provides a next generation, enterprise ready platform for building traditional and cutting-edge content oriented applications. Combining a powerful application development environment with SaaS-based tools and a modular architecture, the Nuxeo Platform and Products provide clear business value to some of the most recognizable brands including Verizon, Electronic Arts, Sharp, FICO, the U.S. Navy, and Boeing. Nuxeo is headquartered in New York and Paris. More information is available at [www.nuxeo.com](http://www.nuxeo.com).

