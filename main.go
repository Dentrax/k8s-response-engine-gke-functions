package kill_the_pwned_pod

import (
	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"context"
	"encoding/json"
	"fmt"
	secretmanagerpb "google.golang.org/genproto/googleapis/cloud/secretmanager/v1"
	"io/ioutil"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"net/http"
	"os"
	"time"
)

// Alert falco data structure
type Alert struct {
	Output       string    `json:"output"`
	Priority     string    `json:"priority"`
	Rule         string    `json:"rule"`
	Time         time.Time `json:"time"`
	OutputFields struct {
		ContainerID              string      `json:"container.id"`
		ContainerImageRepository interface{} `json:"container.image.repository"`
		ContainerImageTag        interface{} `json:"container.image.tag"`
		EvtTime                  int64       `json:"evt.time"`
		FdName                   string      `json:"fd.name"`
		K8SNsName                string      `json:"k8s.ns.name"`
		K8SPodName               string      `json:"k8s.pod.name"`
		ProcCmdline              string      `json:"proc.cmdline"`
	} `json:"output_fields"`
}

var op Operation

type Operation struct {
	clientSet *kubernetes.Clientset
}

// init initializes new Kubernetes ClientSet with given config
func init() {
	// The resource name of the KUBECONFIG_SECRET_NAME in the format
	// `projects/*/secrets/*/versions/*`
	resource := os.Getenv("KUBECONFIG_SECRET_NAME")
	if len(resource) == 0 {
		panic(fmt.Errorf("$KUBECONFIG_SECRET_NAME env variable did not set"))
	}

	secret, err := GetSecret(resource)
	if err != nil {
		panic(fmt.Errorf("get secret: %q", err))
	}

	kubeCfg, err := clientcmd.NewClientConfigFromBytes(secret)
	if err != nil {
		panic(fmt.Errorf("new client config: %q", err))
	}

	restCfg, err := kubeCfg.ClientConfig()
	if err != nil {
		panic(fmt.Errorf("client config: %q", err))
	}

	cs, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		panic(fmt.Errorf("unable to initialize config: %q", err))
	}

	op = Operation{clientSet: cs}
}

// KillThePwnedPod will executed for each Falco event
func KillThePwnedPod(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "cannot read body", http.StatusBadRequest)
		return
	}

	var event Alert

	err = json.Unmarshal(body, &event)
	if err != nil {
		http.Error(w, "cannot parse body", http.StatusBadRequest)
		return
	}

	err = op.PodDestroy(event.OutputFields.K8SPodName, event.OutputFields.K8SNsName)
	if err != nil {
		http.Error(w, fmt.Sprintf("cannot delete pod: %q", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// PodDestroy destroys the given pod name in the given namespace
func (d *Operation) PodDestroy(name, namespace string) error {
	err := d.clientSet.CoreV1().Pods(namespace).Delete(context.TODO(), name, metaV1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("unable to delete pod %s: %q", name, err)
	}
	return nil
}

// GetSecret returns the secret data.
func GetSecret(name string) ([]byte, error) {
	ctx := context.Background()

	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create secretmanager client: %v", err)
	}
	defer client.Close()

	result, err := client.AccessSecretVersion(ctx, &secretmanagerpb.AccessSecretVersionRequest{
		Name: name,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to access secret version: %v", err)
	}

	return result.Payload.Data, nil
}
