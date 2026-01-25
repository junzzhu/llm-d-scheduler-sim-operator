# Hands-On: Deploying LLM-D Stack with Operator

This guide provides the minimal steps to deploy the complete llm-d inference stack (Scheduler, Gateway, Prefill/Decode Simulators) using the operator.

## Prerequisites

1.  **Kubernetes Cluster**: Accessible via `kubectl`.
2.  **Operator Repository**: Cloned locally.

```bash
cd llm-d-scheduler-sim-operator
```

## Step 1: Create Namespace

Create the namespace where the simulator stack will run.

```bash
kubectl create namespace llm-d-sim
kubectl create namespace llm-d-inference-scheduler
```

## Step 2: Install Operator & RBAC

First, install the Custom Resource Definitions (CRDs) and deploy the operator with necessary permissions.

```bash
# 1. Install CRDs
make install

# Verify CRD installation
kubectl get crd simulatordeployments.sim.llm-d.io
kubectl get crd schedulerinstalls.sim.llm-d.io

# 2. Set up RBAC for EPP (Critical)
./hack/create-epp-rbac.sh

# 3. Deploy Operator with fixes
./hack/redeploy-with-fixes.sh
```

*Wait for the operator to initialize.*

## Step 3: Install Gateway API CRDs (Required for Scheduler Routing)

The scheduler workflow uses Gateway API (`Gateway` + `HTTPRoute` + `ReferenceGrant`).

```bash
kubectl apply -k ../llm-d-inference-scheduler/deploy/components/crds-gateway-api
kubectl api-resources | rg -i "gateway|httproute|referencegrant"
```

## Step 4: Deploy the Simulator Stack

Apply the full stack configuration. This single CR creates the EPP, Gateway, and Simulator pods.

```bash
kubectl apply -f config/samples/sim_v1alpha1_simulatordeployment_full.yaml
```

## Step 5: Deploy the Scheduler Stack (SchedulerInstall)

This creates the scheduler EPP, Gateway API Gateway, HTTPRoute, ReferenceGrant, proxy Service, and (optional) DestinationRule.

```bash
kubectl apply -f config/samples/sim_v1alpha1_schedulerinstall.yaml
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
- `infra-inference-scheduling-inference-gateway-istio`: **Running**

## Step 7: Test Connectivity

### Method A: Scheduler Gateway (Recommended)
Forward the scheduler gateway and send inference requests through it.

1.  **Start Port-Forward:**
    ```bash
    kubectl port-forward svc/infra-inference-scheduling-inference-gateway-istio \
      8080:80 -n llm-d-inference-scheduler
    ```

2.  **Run Test (In new terminal):**
    ```bash
    # Test inference
    curl -X POST http://localhost:8080/v1/completions \
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
kubectl port-forward svc/infra-sim-inference-gateway 8080:8080 -n llm-d-sim
```

## Troubleshooting

### 1) `kubectl get gateway,httproute` shows nothing
Gateway API CRDs are not installed.

```bash
kubectl apply -k ../llm-d-inference-scheduler/deploy/components/crds-gateway-api
```

### 2) Gateway shows `PROGRAMMED: False`
Often caused by missing GatewayClass controller or mismatched class name.

```bash
kubectl get gatewayclass
kubectl describe gateway -n llm-d-inference-scheduler infra-inference-scheduling-inference-gateway
```

The default class for `SchedulerInstall` is `istio`. If your cluster only has `kgateway`, set:
```yaml
spec:
  gateway:
    className: kgateway
```

### 3) `no healthy upstream` from gateway
The proxy Service is missing endpoints or the HTTPRoute/ReferenceGrant is misconfigured.

```bash
kubectl get endpointslice -n llm-d-sim \
  -l kubernetes.io/service-name=gaie-inference-scheduling-proxy
kubectl get httproute -n llm-d-inference-scheduler -o yaml
kubectl get referencegrant -n llm-d-sim -o yaml
```

### 4) Check if round-robin is active (Istio)
Verify the DestinationRule load balancer policy:
```bash
kubectl get destinationrule -n llm-d-sim -o jsonpath='{.items[*].spec.trafficPolicy.loadBalancer}'
```

Check the specific rule used by the proxy service:
```bash
kubectl get destinationrule gaie-inference-scheduling-proxy-lb -n llm-d-sim \
  -o jsonpath='{.spec.trafficPolicy.loadBalancer}'
```

If you want to force round-robin explicitly:
```bash
cat <<EOF | kubectl apply -f -
apiVersion: networking.istio.io/v1beta1
kind: DestinationRule
metadata:
  name: gaie-inference-scheduling-proxy-lb
  namespace: llm-d-sim
spec:
  host: gaie-inference-scheduling-proxy.llm-d-sim.svc.cluster.local
  trafficPolicy:
    loadBalancer:
      simple: ROUND_ROBIN
    connectionPool:
      http:
        http1MaxPendingRequests: 1
        maxRequestsPerConnection: 1
EOF
```

### 5) Scheduler EPP crashes (RBAC)
Ensure EPP has permissions for InferencePools and InferenceObjectives:
```bash
kubectl get role,rolebinding -n llm-d-sim | rg gaie-inference-scheduling-epp
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
