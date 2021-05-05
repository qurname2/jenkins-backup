# Jenkins-backup
This simple go code will create a backup of your Jenkins server placed inside Kubernetes.
You need to put path to the kubeconfig via `kubeConfigPath` command line flag.

## Build package
`make build`

## Run locally

    export JENKINS_NAMESPACE=namespace-name
    export S3_REGION=aws-region
    export S3_BUCKET_NAME=bucket-name
    ./out/bin/jenkins-backup -kubeConfigPath=/path/kubeconfig

Aws credentials - you can either use environment variables (AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY) 
or create $HOME/.aws/credentials file.

## Under the hood
* Create k8s client using kubeconfig file
* Create tar.gz archive of jenkins home directory inside remote jenkins pod
* Copy tar archive to localhost and upload to S3 bucket
* Delete created archive from jenkins pod

