# EPP Dev Patch (Live Source Mount)

This directory provides patches to run the EPP directly from your local source tree inside the cluster, avoiding rebuilds. It uses a `minikube mount` hostPath to expose your source to the pod.

## Files

- `epp-dev-patch.yaml` — basic `go run` (simple, slower startup)
- `epp-dev-compile-daemon-patch.yaml` — uses CompileDaemon for auto-restart on file changes
- `epp-dev-revert-patch.yaml` — revert back to the normal image

## Prerequisites

- A running minikube cluster
- The EPP Deployment: `gaie-inference-scheduling-epp` in `llm-d-inference-scheduler`
- Local source at:
  - `~/github/llm-d-scheduler-sim-operator/llm-d-inference-scheduler`

## Step 1: Start the minikube mount

Run this in a separate terminal and keep it open:

```bash
minikube mount ~/github/llm-d-scheduler-sim-operator/llm-d-inference-scheduler:/mnt/epp-src
```

## Step 1.4: Build a dev image with native deps (recommended)

The EPP build requires `python3-dev`, `libzmq3-dev`, and `libtokenizers`. Build a local image and load it into minikube:

```bash
docker build -t epp-dev:local -f epp-dev/Dockerfile epp-dev
minikube image load epp-dev:local
```

## Step 1.5: Persistent Go caches (recommended)

Mount your host Go caches into minikube so module/build downloads persist across pod restarts. Run these in separate terminals and keep them open:

```bash
minikube mount "$(go env GOPATH)/pkg/mod:/mnt/epp-go-mod"
minikube mount "$(go env GOCACHE):/mnt/epp-go-build"
```

## Step 1.6: Pre-download modules on host (recommended if DNS in cluster is flaky)

This avoids Go trying to resolve module hosts from inside the pod.

```bash
cd ~/github/llm-d-scheduler-sim-operator/llm-d-inference-scheduler
GOPROXY=direct go mod download
```

## Step 2: Apply the simple `go run` patch; 

```bash
kubectl patch deploy/gaie-inference-scheduling-epp -n llm-d-inference-scheduler \
  --type merge -p "$(cat epp-dev/epp-dev-patch.yaml)"
  
kubectl rollout restart deploy/gaie-inference-scheduling-epp -n llm-d-inference-scheduler
kubectl rollout status deploy/gaie-inference-scheduling-epp -n llm-d-inference-scheduler
kubectl logs -n llm-d-inference-scheduler deploy/gaie-inference-scheduling-epp -f
```

## Notes

- You would need to **Scale the operator to 0 while you use the dev patch**
- Use following to verify whether step 2A works properly

```bash
  kubectl get deploy/gaie-inference-scheduling-epp -n llm-d-inference-scheduler -o yaml | rg -n "image:|command:|args:|workingDir:|/mnt/epp-src" -C 2
  kubectl exec -n llm-d-inference-scheduler deploy/gaie-inference-scheduling-epp -- pwd
  kubectl exec -n llm-d-inference-scheduler deploy/gaie-inference-scheduling-epp -- ls -la /mnt/epp-src

#  With expected output
#
#  - image: golang:1.24
#  - workingDir: /mnt/epp-src
#  - go run ./cmd/epp/main.go
```

- This replaces the image with `golang:1.24` and runs directly from `/mnt/epp-src`.
- If the pod starts with missing modules, it will download them at startup.
- CompileDaemon uses polling (`-polling`) to handle filesystem events reliably with hostPath.

## Revert to the normal image

```bash
kubectl patch deploy/gaie-inference-scheduling-epp -n llm-d-inference-scheduler \
  --type merge -p "$(cat epp-dev/epp-dev-revert-patch.yaml)"

kubectl rollout restart deploy/gaie-inference-scheduling-epp -n llm-d-inference-scheduler
kubectl rollout status deploy/gaie-inference-scheduling-epp -n llm-d-inference-scheduler
```

## Optional: Set POOL_NAME/POOL_NAMESPACE

If you need to override `POOL_NAME` or `POOL_NAMESPACE`, edit the patch file and replace
`${POOL_NAME}` / `${POOL_NAMESPACE}` with explicit values.

## Success criteria (expected logs)

When EPP is running correctly with body processing enabled, you should see lines like:

```
ext_proc request body chunk received
ext_proc send request body responses
```

You can tail logs with:

```bash
kubectl logs -n llm-d-inference-scheduler deploy/gaie-inference-scheduling-epp -f | rg -n "request body|send request body responses"
```

Quick sanity curl (replace `$GATEWAY_IP`):

```bash
export GATEWAY_IP=$(kubectl get svc -n llm-d-inference-scheduler -l gateway.networking.k8s.io/gateway-name=infra-inference-scheduling-inference-gateway -o jsonpath='{.items[0].spec.clusterIP}')
kubectl run -n llm-d-inference-scheduler curl-gw --rm -i --restart=Never --image=curlimages/curl -- \
  -m 15 --connect-timeout 3 -v \
  http://$GATEWAY_IP/v1/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"random","prompt":"test","max_tokens":5}'
```

## Re-enable the operator (if you scaled it down)

```bash
kubectl scale deploy/<operator-name> -n <operator-namespace> --replicas=1
```
