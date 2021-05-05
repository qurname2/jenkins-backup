package k8sUtils

import (
	"bytes"
	"context"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io"
	core_v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

// K8sClient holds a clientset and a config
type K8sClient struct {
	ClientSet *kubernetes.Clientset
	Config    *rest.Config
}

//GetK8sClients get a k8s client from local kube config file
func GetClientToK8s(kubeconfig string) (*K8sClient, error) {

	var config *rest.Config

	_, err := os.Stat(kubeconfig)
	if err != nil {
		// In cluster configuration - todo
		//config, err = rest.InClusterConfig()
		//if err != nil {
		//	return nil, err
		//}
		log.Printf("some error occurred %s", err)
		os.Exit(1)
	} else {
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, err
		}
	}

	clientset, errNewForConfig := kubernetes.NewForConfig(config)
	if errNewForConfig != nil {
		return nil, errNewForConfig
	}
	var client = &K8sClient{ClientSet: clientset, Config: config}
	return client, nil
}

//GetReadyPodName get a list of ready pods according to defined namespace and selector
func GetReadyPodName(ik8sClient interface{}, namespace, selector string) ([]string, error) {

	k8sClient := *ik8sClient.(*K8sClient)

	options := metav1.ListOptions{
		LabelSelector: selector,
	}
	pods, err := k8sClient.ClientSet.CoreV1().Pods(namespace).List(context.TODO(), options)
	if err != nil {
		return nil, err
	}

	var podList []string
	for _, pod := range pods.Items {
		ready := true
		// We need pods with correct status only
		// https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/#pod-conditions
		for _, condition := range pod.Status.Conditions {
			if condition.Status != "True" {
				ready = false
				break
			}
		}
		if ready {
			podList = append(podList, pod.Name)
		}
	}

	return podList, nil
}

//executeIntoK8s interactively exec to the pod through API with the specified command.
func executeIntoK8s(ik8sClient interface{}, namespace, podName, containerName string, command []string, stdin io.Reader, stdout io.Writer) ([]byte, error) {

	k8sClient := *ik8sClient.(*K8sClient)

	req := k8sClient.ClientSet.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec")

	scheme := runtime.NewScheme()
	if err := core_v1.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("error adding to scheme: %v", err)
	}

	parameterCodec := runtime.NewParameterCodec(scheme)
	req.VersionedParams(&core_v1.PodExecOptions{
		Command:   command,
		Container: containerName,
		Stdin:     stdin != nil,
		Stdout:    true,
		Stderr:    true,
		TTY:       true,
	}, parameterCodec)

	log.Debugf("exec into pod: %s, container: %s, ns: %s, command: %s", podName, containerName, namespace, req.URL() )

	exec, err := remotecommand.NewSPDYExecutor(k8sClient.Config, "POST", req.URL())
	if err != nil {
		return nil, fmt.Errorf("error while creating Executor: %v", err)
	}

	var stderr bytes.Buffer
	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: &stderr,
	})
	if err != nil {
		return nil, fmt.Errorf("error in Stream: %v", err)
	}
	return stderr.Bytes(), nil
}

//KubectlCp copy file from remote pod to localhost:dstPath
func KubectlCp(pod, dstPath, namespace, containerName, kubeConfigPath string) error {

	podWithPath := fmt.Sprintf("%s:%s", pod, dstPath)
	kubectlExec, _ := exec.LookPath("kubectl")
	log.Debugf("pod name with path of tar archive is: %s", podWithPath)

	cmdKubectlCp := &exec.Cmd{
		Path: kubectlExec,
		Args: []string{kubectlExec, "cp", "-n", namespace, "-c", containerName, podWithPath, dstPath, "--kubeconfig", kubeConfigPath},
	}

	if _, err := cmdKubectlCp.Output(); err != nil {
		return fmt.Errorf("error occured with kubectl cp command: %v", err)
	} else {
		log.Infof("tar archive was copied from remote jenkins pod to localhost:%s", dstPath)
		return nil
	}
}

//CreateTarK8sPod will create tar.gz archive of jenkins_home directory
func CreateTarK8sPod(ik8sClient interface{}, namespace, pod, containerName, dstPath, srcPath string, stdout io.Writer) {

	cmdExec := []string{"/bin/tar", "--ignore-failed-read", "-cvzf", dstPath, "--exclude-vcs", "--exclude=/var/jenkins_home/plugins", "--exclude=/var/jenknis_home/casc_configs", "--exclude=/var/jenkins_home/war", "--exclude=/var/jenkins_home/secret*", "--exclude=/var/jenkins_home/log*", "--exclude=/var/jenkins_home/caches", "--warning=no-file-changed", srcPath}
	stderr, err := executeIntoK8s(ik8sClient, namespace, pod, containerName, cmdExec, nil, stdout)

	if len(stderr) != 0 {
		log.Infof("STDERR: %s" + (string)(stderr))
	}

	if err != nil {
		// Will check if command for Execute - creation of tar archive and return code is 1.
		// In that case will continue the program execution, because tar can be finished with error code 1 due to some file was changed during the tar archive creation.
		stringFromArrCmdExecution := strings.Join(cmdExec, ", ")
		reTar := regexp.MustCompile(`(.?)tar`)
		if len(reTar.FindStringSubmatch(stringFromArrCmdExecution)) != 0 {
			reExitCode := regexp.MustCompile(`exit code 1`)
			if len(reExitCode.FindStringSubmatch(err.Error())) != 0 {
				log.Info("creation of tar archive was finished with exit code = 1, will continue the program execution")
			} else {
				log.Fatal(err)
			}
		} else {
			log.Fatal(err)
		}
	}
}

//DeleteArchiveK8s delete archive from remote pod after successful upload to S3 bucket
func DeleteArchiveK8s(ik8sClient interface{}, namespace, pod, containerName, dstPath string, stdout io.Writer) {

	cmdRm := []string{"/bin/rm", "-rf", dstPath}
	stderr, err := executeIntoK8s(ik8sClient, namespace, pod, containerName, cmdRm, nil, stdout)
	if len(stderr) != 0 {
		log.Infof("STDERR: %s" + (string)(stderr))
	}
	if err != nil {
		log.Fatal(err)
	}
	log.Info("archive was deleted from remote Jenkins pod")
}
