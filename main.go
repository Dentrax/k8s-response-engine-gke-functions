package kill_the_pwned_pod

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"net/http"
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
	kubeCfg := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{},
	)

	restCfg, err := kubeCfg.ClientConfig()
	if err != nil {
		panic(err)
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
