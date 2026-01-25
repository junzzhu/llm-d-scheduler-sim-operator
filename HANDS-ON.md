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
```

## Step 2: Install Operator & RBAC

First, install the Custom Resource Definitions (CRDs) and deploy the operator with necessary permissions.

```bash
# 1. Install CRDs
make install

# Verify CRD installation
kubectl get crd simulatordeployments.sim.llm-d.io

# 2. Set up RBAC for EPP (Critical)
./hack/create-epp-rbac.sh

# 3. Deploy Operator with fixes
./hack/redeploy-with-fixes.sh
```

*Wait for the operator to initialize.*

## Step 3: Deploy the Stack

Apply the full stack configuration. This single CR creates the EPP, Gateway, and Simulator pods.

```bash
kubectl apply -f config/samples/sim_v1alpha1_simulatordeployment_full.yaml
```

## Step 4: Verify Deployment

Wait for all pods to be ready:

```bash
kubectl get pods -n llm-d-sim -w
```

**Expected Output:**
- `gaie-sim-epp`: **Running**
- `infra-sim-inference-gateway`: **Running**
- `ms-sim-llm-d-modelservice-decode`: **Running** (2 replicas)
- `ms-sim-llm-d-modelservice-prefill`: **Running** (2 replicas)

## Step 5: Test Connectivity

### Method A: OpenShift Route (Production)
If running on OpenShift, use the exposed route:
```bash
ROUTE_URL=$(oc get route infra-sim-inference-gateway -n llm-d-sim -o jsonpath='{.spec.host}')
curl http://${ROUTE_URL}/v1/models
```

### Method B: Port-Forward (Minikube/Kind)
If running locally, forward the gateway port:

1.  **Start Port-Forward:**
    ```bash
    # IMPORTANT: Map local 8080 to remote 8080 (Service Port)
    kubectl port-forward svc/infra-sim-inference-gateway 8080:8080 -n llm-d-sim
    ```

2.  **Run Test (In new terminal):**
    ```bash
    # List models
    curl http://localhost:8080/v1/models

    # Test inference
    curl -X POST http://localhost:8080/v1/completions \
      -H "Content-Type: application/json" \
      -d '{"model": "random", "prompt": "test", "max_tokens": 5}'
    ```

**Expected Response (Inference):**
```json
{"id":"cmpl-909b27c2-7b55-5bbe-998f-16c36d477595","created":1769310221,"model":"random","usage":{"prompt_tokens":1,"completion_tokens":5,"total_tokens":6},"object":"text_completion",...}
```

## Clean Up

To remove the deployment and CRDs:

```bash
# Delete the deployment
kubectl delete simulatordeployment llm-sim-full -n llm-d-sim

# (Optional) Delete CRDs to fully clean up
make uninstall

# Delete the namespace
kubectl delete namespace llm-d-sim
```
