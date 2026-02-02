# Minikube (IPVS) Setup and End-to-End Test

This guide walks through creating a fresh minikube cluster with kube-proxy in
IPVS mode and validating the end-to-end scheduler + inference pool flow.

## Quick start (IPVS-ready minikube)

```bash
minikube start --extra-config=kube-proxy.mode=ipvs
# or
minikube start --kube-proxy=ipvs
```

Verify:
```bash
kubectl logs -n kube-system -l k8s-app=kube-proxy --tail=50 | grep -i ipvs
```

Expected log:
```
Using ipvs Proxier
```

If you do not see IPVS in the logs, load the kernel modules and restart
kube-proxy:

```bash
minikube ssh -- "sudo modprobe ip_vs ip_vs_rr ip_vs_wrr ip_vs_sh nf_conntrack"
kubectl rollout restart ds/kube-proxy -n kube-system
kubectl logs -n kube-system -l k8s-app=kube-proxy --tail=50 | grep -i ipvs
```

## 1) Create a new minikube cluster with IPVS

```bash
# Optional: back up any existing resources
kubectl get all -A -o yaml > /tmp/llm-d-sim-backup.yaml

# Recreate minikube with kube-proxy in IPVS mode
minikube delete
minikube start --extra-config=kube-proxy.mode=ipvs

# Verify kube-proxy is using IPVS
kubectl logs -n kube-system -l k8s-app=kube-proxy --tail=50 | grep -i ipvs
```

Expected log:
```
Using ipvs Proxier
```

## 2) Load Docker Images into Minikube (Optional)

If you have local Docker images or want to avoid pulling from registries during deployment, load them into minikube beforehand. This prevents image pull errors and speeds up deployment.

### Option A: Configure Minikube to Use Docker Driver (Recommended for verification)

Start minikube with Docker as the container runtime driver. This allows minikube to directly use images from your host Docker daemon:

```bash
# Start minikube with Docker driver
minikube start --driver=docker --extra-config=kube-proxy.mode=ipvs

# Verify minikube is using Docker driver
minikube profile list
# Should show driver: docker
```

With this configuration, minikube can access images from your host Docker directly. Verify images are available:

```bash
# Check images in your host Docker
docker images | grep -E "llm-d-simulator|llm-d-inference-scheduler|proxyv2|envoy-wrapper"

# These images will be accessible to minikube pods automatically
```

### Option B: Load Images from Host Docker into Minikube

Load images from your host Docker into minikube:

```bash
# Load simulator image (if built locally)
minikube image load llm-d-simulator:local

# Pull and load scheduler image from registry
docker pull ghcr.io/llm-d/llm-d-inference-scheduler:v0.4.0
minikube image load ghcr.io/llm-d/llm-d-inference-scheduler:v0.4.0

# Optional: Load Istio proxy image (if using Istio gateway)
docker pull docker.io/istio/proxyv2:1.28.3
minikube image load docker.io/istio/proxyv2:1.28.3

# Optional: Load kgateway image (if using standard gateway)
docker pull cr.kgateway.dev/kgateway-dev/envoy-wrapper:v2.1.1
minikube image load cr.kgateway.dev/kgateway-dev/envoy-wrapper:v2.1.1
```

### Verify Images are Available

```bash
# List images in minikube
minikube image ls | grep -E "llm-d-simulator|llm-d-inference-scheduler|proxyv2|envoy-wrapper"

# Or if using minikube's Docker daemon (Option A)
eval $(minikube docker-env)
docker images | grep -E "llm-d-simulator|llm-d-inference-scheduler|proxyv2|envoy-wrapper"
eval $(minikube docker-env -u)
```

## 3) Install Gateway Inference Extension CRDs

The Gateway Inference Extension CRDs must be installed before starting the operator and deploying resources.

```bash
# Install CRDs from the sibling llm-d-inference-scheduler repository
kubectl apply -k ../llm-d-inference-scheduler/deploy/components/crds-gie
```

Verify the CRDs are installed:
```bash
kubectl get crd inferenceobjectives.inference.networking.x-k8s.io
kubectl get crd inferencepools.inference.networking.k8s.io
```

## 4) Reduce Memory Requests (for memory-constrained environments)

If running on a memory-constrained minikube (e.g., 2GB), you can reduce the simulator pod memory requests after deployment. This step can be done after applying the SimulatorDeployment in step 5.

```bash
# Reduce decode deployment memory from 256Mi to 128Mi
kubectl patch deployment ms-sim-llm-d-modelservice-decode -n llm-d-sim --type='json' \
  -p='[{"op": "replace", "path": "/spec/template/spec/containers/0/resources/requests/memory", "value": "128Mi"}]'

# Reduce prefill deployment memory from 256Mi to 128Mi
kubectl patch deployment ms-sim-llm-d-modelservice-prefill -n llm-d-sim --type='json' \
  -p='[{"op": "replace", "path": "/spec/template/spec/containers/0/resources/requests/memory", "value": "128Mi"}]'
```

Wait for pods to restart with new memory settings:
```bash
kubectl get pods -n llm-d-sim -w
```

## 8) Create InferencePool

```bash
cat <<'EOF' | kubectl apply -f -
apiVersion: inference.networking.k8s.io/v1
kind: InferencePool
metadata:
  name: gaie-inference-scheduling
  namespace: llm-d-sim
spec:
  endpointPickerRef:
    group: ""
    kind: Service
    name: gaie-sim-epp
    port:
      number: 8100
  selector:
    matchLabels:
      llm-d.ai/role: decode
      llm-d.ai/inferenceServing: "true"
  targetPorts:
  - number: 8200
EOF
```

## 9) Enable inference extension in Istio

```bash
kubectl -n istio-system set env deploy/istiod ENABLE_GATEWAY_API_INFERENCE_EXTENSION=true
kubectl rollout restart deploy/istiod -n istio-system
```

## 10) Verify routing objects

```bash
kubectl get gateway -n llm-d-inference-scheduler
kubectl get httproute -n llm-d-inference-scheduler -o yaml | rg -n "InferencePool|backendRefs"
kubectl get httproute -n llm-d-inference-scheduler -o yaml | rg -n "Accepted|ResolvedRefs|message"

kubectl get referencegrant -A | rg -n "InferencePool"
```

## 11) Verify EnvoyFilter + ext_proc

```bash
kubectl get envoyfilter -n llm-d-inference-scheduler
kubectl exec -n llm-d-inference-scheduler <gateway-pod> -- \
  pilot-agent request GET config_dump | rg -n "ext_proc|epp-cluster"
```

## 12) Backend sanity check (direct to pod IP)

```bash
kubectl get pods -n llm-d-sim -l llm-d.ai/role=decode -o wide
kubectl run -n llm-d-sim curl-pod --rm -i --restart=Never --image=curlimages/curl -- \
  -m 5 --connect-timeout 2 -v http://<POD_IP>:8200/v1/completions \
  -H "Content-Type: application/json" -d '{"model":"random","prompt":"test","max_tokens":5}'
```

## 13) End-to-end request through the gateway

```bash
kubectl get svc -n llm-d-inference-scheduler infra-inference-scheduling-inference-gateway-istio -o jsonpath='{.spec.clusterIP}'; echo

kubectl run -n llm-d-inference-scheduler curl-gw --rm -i --restart=Never --image=curlimages/curl -- \
  -m 15 --connect-timeout 3 -v \
  http://<GATEWAY_CLUSTER_IP>/v1/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"random","prompt":"test","max_tokens":5}'
```

## 14) Check EPP logs

```bash
kubectl logs -n llm-d-inference-scheduler deploy/gaie-inference-scheduling-epp --tail=50 | rg -n "checking|score|decision|error|warn"
kubectl logs -n llm-d-sim deploy/gaie-sim-epp --tail=50 | rg -n "checking|score|decision|error|warn"
```


kubectl patch schedulerinstall llm-sched-install -n llm-d-inference-scheduler --type merge -p \
    '{"spec":{"epp":{"verbosity":5}}}'



Fixes Applied
Fix 1: Changed EnvoyFilter Operation
kubectl patch envoyfilter epp-ext-proc -n llm-d-inference-scheduler --type=json -p='[
  {"op": "replace", "path": "/spec/configPatches/0/patch/operation", "value": "REPLACE"},
  {"op": "replace", "path": "/spec/configPatches/0/match/listener/filterChain/filter/subFilter/name", "value": "envoy.filters.http.ext_proc"}
]'

Result: HTTP filter chain now has ext_proc with cluster_name: "epp-cluster" and request_header_mode: "SEND" ✅

Fix 2: Changed Request Body Mode
kubectl patch envoyfilter epp-ext-proc -n llm-d-inference-scheduler --type=json -p='[
  {"op": "replace", "path": "/spec/configPatches/0/patch/value/typed_config/processing_mode/request_body_mode", "value": "NONE"},
  {"op": "replace", "path": "/spec/configPatches/2/patch/value/typed_per_filter_config/envoy.filters.http.ext_proc/overrides/processing_mode/request_body_mode", "value": "NONE"}
]'

Fix 3: Restarted Gateway Pod
kubectl delete pod <gateway-pod> -n llm-d-inference-scheduler



Next Steps (After Pod Restart)
Verify ext_proc is working:

# Send test request
kubectl exec <new-gateway-pod> -- curl http://localhost:80/v1/completions -d '{"model":"random","prompt":"test","max_tokens":5}'

# Check ext_proc stats (should be > 0)
kubectl exec <new-gateway-pod> -- pilot-agent request GET stats | grep ext_proc

# Check EPP logs (should show processing)
kubectl logs deploy/gaie-inference-scheduling-epp -n llm-d-inference-scheduler --tail=50

If still no logs, increase verbosity:

kubectl patch schedulerinstall llm-sched-install -n llm-d-inference-scheduler --type merge -p '{"spec":{"epp":{"verbosity":9}}}'