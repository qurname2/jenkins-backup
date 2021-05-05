package main

import (
	"bytes"
	"flag"
	"github.com/qurname2/jenkins-backup/pkg/k8sUtils"
	"github.com/qurname2/jenkins-backup/pkg/utils"
	log "github.com/sirupsen/logrus"
	"os"
)

type backupSettings struct {
	namespace		string
	selector   		string
	containerName	string
	srcPath			string
	dstPath			string
}

const (
	srcPath = "/var/jenkins_home"
	dstPath = "/tmp/jenkins_home.tar.gz"
)

var kubeConfigPath string

func init() {
	// Log as JSON instead of the default ASCII formatter.
	log.SetFormatter(&log.JSONFormatter{})

	// Output to stdout instead of the default stderr
	log.SetOutput(os.Stdout)

	// Only log the info severity or above.
	log.SetLevel(log.InfoLevel)

	// Command line flag for kube config
	flag.StringVar(&kubeConfigPath, "kubeConfigPath", "", "there path to your local kube config file should be specified")
	flag.Parse()
	if len(kubeConfigPath) == 0 {
		log.Info("Usage: main.go -kubeConfigPath")
		flag.PrintDefaults()
		os.Exit(1)
	}
}

func main()  {
	assertEnvs()
	log.Info("all assertion passed, start jenkins backup")

	backupSet := &backupSettings{
		namespace: os.Getenv("JENKINS_NAMESPACE"),
		selector: "app.kubernetes.io/component=jenkins-controller",
		containerName: "jenkins",
		srcPath: srcPath,
		dstPath: dstPath,
	}

	k8sClient, err := k8sUtils.GetClientToK8s(kubeConfigPath)
	if err != nil {
		utils.ExitErrorf("error occurred during getting k8sClient: ", err)
	}
	log.Debug("got k8s client")

	pods, errGetReadyPodName := k8sUtils.GetReadyPodName(k8sClient, backupSet.namespace, backupSet.selector)
	if errGetReadyPodName != nil {
		utils.ExitErrorf(errGetReadyPodName)
	}
	if len(pods) == 0 {
		utils.ExitErrorf("something unpredictable happened, list of pods is empty, please check either .kube/config or namespace or selector definition")
	}

	log.Debugf("here is the pod list ready for backup: %s", pods)

	stdout := new(bytes.Buffer)

	// We have k8s Client, pod list, time for tar archive creation
	k8sUtils.CreateTarK8sPod(k8sClient, backupSet.namespace, pods[0], backupSet.containerName, backupSet.dstPath, backupSet.srcPath, stdout)

	// Download archive to localhost
	errKubectlCp := k8sUtils.KubectlCp(pods[0], backupSet.dstPath, backupSet.namespace, backupSet.containerName, kubeConfigPath)
	if errKubectlCp != nil {
		log.Fatal(errKubectlCp)
	}

	// Upload archive from localhost to S3 bucket
	errUpload := utils.S3UploadObject(os.Getenv("S3_BUCKET_NAME"), backupSet.dstPath)
	if errUpload != nil {
		utils.ExitErrorf("error occured during S3 upload")
	}

	// Upload to S3 was successful, now we need to remove archive from Jenkins pod
	k8sUtils.DeleteArchiveK8s(k8sClient, backupSet.namespace, pods[0], backupSet.containerName, backupSet.dstPath, stdout)

	log.Info("finished successfully, bye!")
}

//assertEnvs checks availability of needed envs
func assertEnvs() {
	listNeededEnvs := []string{"JENKINS_NAMESPACE", "S3_REGION", "S3_BUCKET_NAME"}
	for _,v := range listNeededEnvs{
		_, present := os.LookupEnv(v)
		if ! present {
			utils.ExitErrorf("environment variable must be set: ", v)
		}
	}
}
