# Kubernetes Respons Engine on GKE by using Google Cloud Functions, Falco and Falcosidekick

![arch](./assets/gcloudfalcov2.png)

> Similar: https://github.com/developer-guy/google-cloud-function-stdout-falco-alert

A simple demo about how to set up Kubernetes Respons Engine on GKE by using Google Cloud Functions, Falco and Falcosidekick

### Prerequisites

* gcloud 342.0.0

### Tutorial

To test workloadidentity first create a GCP cluster with workloadidentity enabled

```bash
$ GOOGLE_PROJECT_ID=$(gcloud config get-value project)
$ CLUSTER_NAME=falco-falcosidekick-demo
$ gcloud container clusters create $CLUSTER_NAME \
                   --workload-pool ${GOOGLE_PROJECT_ID}.svc.id.goog
```

Let's deploy the Google Cloud Functions first, because in the later steps, we'll need the name of the function.

```bash
$ git clone kubernetes-response-engine-based-on-gke-and-gcloudfunctions
$ cd kubernetes-response-engine-based-on-gke-and-gcloudfunctions
$ export FUNCTION_NAME=KillThePwnedPod
$ gcloud functions deploy $FUNCTION_NAME --runtime go113 --trigger-http
Allow unauthenticated invocations of new function [KillThePwnedPod]? (y/N)? N
...
```

Get the name of the function
```bash
$ CLOUD_FUNCTION_NAME=$(gcloud functions describe --format=json $FUNCTION_NAME | jq -r '.name')
```

Once it's created, lets install `Falco`, and `Falcosidekick` with enabled `Google Cloud Functions` output type.

```bash
$ export FALCO_NAMESPACE=falco
$ kubectl create namespace $FALCO_NAMESPACE
$ helm upgrade --install falco falco \
--namespace $FALCO_NAMESPACE \
--set ebpf.enabled=true \
--set falcosidekick.enabled=true \
--set falcosidekick.config.gcp.cloudfunctions.name=${CLOUD_FUNCTION_NAME} \
--set falcosidekick.webui.enabled=true
```

Finally set up the your SA and Rolebindings
```bash
$ SA_ACCOUNT=falco-falcosidekick-sa
$ gcloud iam service-accounts create $SA_ACCOUNT

$ gcloud projects add-iam-policy-binding ${GOOGLE_PROJECT_ID} \
--member="serviceAccount:${SA_ACCOUNT}@${GOOGLE_PROJECT_ID}.iam.gserviceaccount.com" \
--role="roles/cloudfunctions.developer"

$ gcloud projects add-iam-policy-binding ${GOOGLE_PROJECT_ID} \
--member="serviceAccount:${SA_ACCOUNT}@${GOOGLE_PROJECT_ID}.iam.gserviceaccount.com" \
--role="roles/cloudfunctions.invoker"

$ gcloud iam service-accounts add-iam-policy-binding \
  --role roles/iam.workloadIdentityUser \
  --member "serviceAccount:${GOOGLE_PROJECT_ID}.svc.id.goog[${FALCO_NAMESPACE}/falco-falcosidekick]" \
  ${SA_ACCOUNT}@${GOOGLE_PROJECT_ID}.iam.gserviceaccount.com
```

Finally set up the Falcosidekick SA to impersonate a GCP SA
```bash
$ kubectl annotate serviceaccount \
  --namespace $FALCO_NAMESPACE \
  falco-falcosidekick \
  iam.gke.io/gcp-service-account=${SA_ACCOUNT}@${GOOGLE_PROJECT_ID}.iam.gserviceaccount.com
```

### Test

Create an alpine pod first, then try to exec into it.

```bash
$ kubectl run alpine  --image=alpine --restart='Never' -- sh -c "sleep 600"
```

Exec into it.
```bash
$ kubectl exec -i --tty alpine -- sh -c "uptime"
```
