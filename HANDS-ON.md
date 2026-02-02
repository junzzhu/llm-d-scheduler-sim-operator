# Hands-On: Deploying LLM-D Stack with Operator

This guide provides the minimal steps to deploy the llm-d inference stack (Scheduler, Gateway, Prefill/Decode Simulators) using the operator.

## Prerequisites

1.  **Kubernetes Cluster**: Accessible via `kubectl` with sufficient CPU/memory.
    - For Minikube setup (including IPVS mode and recommended sizing), follow `doc/minikube-ipvs-setup.md`.
2.  **Operator Repository**: Cloned locally.
3.  **[llm-d-inference-scheduler](https://github.com/llm-d/llm-d-inference-scheduler)** Cloned locally as a sibling repo for above.
4.  **Istio**: Required for Gateway API functionality (installed in Step 2).

```bash
cd llm-d-scheduler-sim-operator
```

## Step 1: Create Namespace

Create the namespace where the simulator stack will run.

```bash
kubectl create namespace llm-d-sim
kubectl create namespace llm-d-inference-scheduler
```

## Step 2: Install Istio

Istio is required for the Gateway API functionality. Install it before deploying the operator.

```bash
# Install Istio with minimal profile (suitable for testing)
./istio-1.28.3/bin/istioctl install --set profile=minimal -y

# Verify Istio installation
kubectl get pods -n istio-system

# Expected output: istiod pod should be Running
```

**Note:** If you encounter memory issues with istiod, you can reduce its memory requirements:
```bash
kubectl patch deployment istiod -n istio-system --type='json' \
  -p='[{"op": "replace", "path": "/spec/template/spec/containers/0/resources/requests/memory", "value": "128Mi"}]'
```

## Step 3: Install Operator & RBAC

Install the Custom Resource Definitions (CRDs) and deploy the operator with necessary permissions.

```bash
# 1. Install CRDs
make install

# Verify CRD installation
kubectl get crd simulatordeployments.sim.llm-d.io
kubectl get crd schedulerinstalls.sim.llm-d.io

# Apply the full stack configuration. This single CR creates the EPP, Gateway, and Simulator pods.
kubectl apply -f config/samples/sim_v1alpha1_simulatordeployment_full.yaml -n llm-d-sim

# 2. Set up RBAC for EPP
./script/create-epp-rbac.sh
```

*Wait for the operator to initialize.*

## Step 4: Install Gateway API CRDs (Required for Scheduler Routing)

The scheduler workflow uses Gateway API (`Gateway` + `HTTPRoute` + `ReferenceGrant`).

```bash
kubectl apply -k ../llm-d-inference-scheduler/deploy/components/crds-gateway-api
kubectl api-resources | rg -i "gateway|httproute|referencegrant"
```

## Step 5: Deploy the Simulator and Scheduler Stack

```bash
# Deploy Operator with fixes (optional, if necessary)
./script/redeploy-with-fixes.sh
```

This creates the scheduler EPP, Gateway API Gateway, HTTPRoute, ReferenceGrant, proxy Service, and (optional) DestinationRule.

```bash
kubectl apply -f config/samples/sim_v1alpha1_schedulerinstall.yaml
```

Start operator.

```bash
./bin/manager > /tmp/operator.log
```

## Step 6: Verify Deployment

Wait for all pods to be ready:

```bash
kubectl get pods -n llm-d-sim -w
kubectl get pods -n llm-d-inference-scheduler -w
```

**Expected Output:**
- `gaie-sim-epp`: **Running**
- `infra-sim-inference-gateway`: **Running**
- `ms-sim-llm-d-modelservice-decode`: **Running** (2 replicas)
- `ms-sim-llm-d-modelservice-prefill`: **Running** (2 replicas)
- `gaie-inference-scheduling-epp`: **Running**
- Istio Gateway pods (created automatically): **Running** -  `kubectl get gatewayclass`


**Note:** The Gateway resource `infra-inference-scheduling-inference-gateway` automatically creates a Kubernetes Service named `infra-inference-scheduling-inference-gateway-istio` (with `-istio` suffix added by Istio). This service is used for port-forwarding in the next step.

Verify the gateway service exists:
```bash
kubectl get svc -n llm-d-inference-scheduler infra-inference-scheduling-inference-gateway-istio
```

## Step 7: Test Connectivity

### Method A: Scheduler Gateway (Recommended)
Forward the scheduler gateway and send inference requests through it.

1.  **Start Port-Forward:**
    ```bash
    kubectl port-forward svc/infra-inference-scheduling-inference-gateway-istio \
      18080:80 -n llm-d-inference-scheduler
    ```

2.  **Run Test (In new terminal):**
    ```bash
    # Test inference
    curl -X POST http://localhost:18080/v1/completions \
      -H "Content-Type: application/json" \
      -d '{"model": "random", "prompt": "test", "max_tokens": 5}'
    ```

**Expected Response (Inference):**
```json
{"id":"cmpl-909b27c2-7b55-5bbe-998f-16c36d477595","created":1769310221,"model":"random","usage":{"prompt_tokens":1,"completion_tokens":5,"total_tokens":6},"object":"text_completion",...}
```

### Method B: Direct Simulator Gateway (Optional)
If you want to bypass the scheduler and hit the simulator gateway directly:

```bash
kubectl port-forward svc/infra-sim-inference-gateway 18081:8080 -n llm-d-sim
```

## Round-robin scheduling: verify distribution across decode pods

This test verifies that Istio is using ROUND_ROBIN (instead of the LEAST_REQUEST) in our setup for the proxy Service and requests are distributed across decode pods. Expect **both decode pods** to receive traffic. Due to HTTP keep-alive, the distribution may not be perfectly alternating; the key signal is that **both pods log requests over time**.

Verify the DestinationRule load balancer policy is `ROUND_ROBIN`:

```bash
kubectl get destinationrule -n llm-d-sim -o jsonpath='{.items[*].spec.trafficPolicy.loadBalancer}'
{"simple":"ROUND_ROBIN"}
```

```bash
kubectl get endpointslice -n llm-d-sim \
  -l kubernetes.io/service-name=gaie-inference-scheduling-proxy
NAME                                    ADDRESSTYPE   PORTS   ENDPOINTS                   AGE
gaie-inference-scheduling-proxy-rll2v   IPv4          8200    10.244.0.247,10.244.0.248   18m

kubectl get pods -n ${NS_SIM} -l llm-d.ai/role=decode
NAME                                                READY   STATUS    RESTARTS   AGE
ms-sim-llm-d-modelservice-decode-6dd4d89c5d-jxgrv   1/1     Running   0          15m
ms-sim-llm-d-modelservice-decode-6dd4d89c5d-r6hjn   1/1     Running   0          15m

kubectl logs ms-sim-llm-d-modelservice-decode-6dd4d89c5d-jxgrv -n ${NS_SIM} -f | rg worker
kubectl logs ms-sim-llm-d-modelservice-decode-6dd4d89c5d-r6hjn -n ${NS_SIM} -f | rg worker
```

Expected behavior, with following client requests:
- Both decode pod logs show request handling lines after sending traffic.
- If only one pod logs requests, check keep-alive settings or the DestinationRule.

```bash
for i in {1..8}; do
  curl -X POST http://localhost:18080/v1/completions \
    -H "Content-Type: application/json" \
    -d '{"model": "random", "prompt": "test", "max_tokens": 5}'
  sleep 0.5
done
```

## Clean Up

To remove the deployment and CRDs:

```bash
# Delete the deployment
kubectl delete simulatordeployment llm-sim-full -n llm-d-sim
kubectl delete schedulerinstall llm-sched-install -n llm-d-inference-scheduler

# (Optional) Delete CRDs to fully clean up
make uninstall

# Delete the namespace
kubectl delete namespace llm-d-sim
kubectl delete namespace llm-d-inference-scheduler
```

## Debug Image Rollout

For rebuilding and rolling out the EPP debug image, see `doc/development.md`.
