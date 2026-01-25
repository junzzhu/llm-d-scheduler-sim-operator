# SimulatorDeployment Sample Configurations

This directory contains sample Custom Resources (CRs) for deploying llm-d simulators using the operator.

## Available Samples

### 1. Minimal Configuration
**File:** `sim_v1alpha1_simulatordeployment_minimal.yaml`

Minimal configuration with just replica count specified. All other settings use defaults.

```bash
kubectl apply -f sim_v1alpha1_simulatordeployment_minimal.yaml
```

**Features:**
- 2 replicas
- Default image: `ghcr.io/llm-d/llm-d-simulator:latest`
- Default service: `gaie-inference-scheduling-proxy` on port 8200
- ClusterIP service type
- No custom load balancing

**Use Cases:**
- Quick testing
- Development environments
- Simple deployments

---

### 2. Istio with Load Balancing
**File:** `sim_v1alpha1_simulatordeployment_istio.yaml`

Complete configuration with Istio gateway and custom load balancing.

```bash
kubectl apply -f sim_v1alpha1_simulatordeployment_istio.yaml
```

**Features:**
- 2 replicas
- Istio gateway enabled
- Round-robin load balancing
- Connection pool settings
- Custom service configuration

**Use Cases:**
- Production deployments
- Service mesh environments
- Testing load balancing algorithms

---

## Quick Start

### Prerequisites

1. **Create namespace:**
```bash
kubectl create namespace llm-d-sim
```

2. **Install operator CRDs:**
```bash
kubectl apply -f ../crd/
```

3. **Run operator:**
```bash
# From project root
make run
```

### Deploy a Sample

**For simple testing:**
```bash
kubectl apply -f sim_v1alpha1_simulatordeployment_minimal.yaml
```

**For Istio environments:**
```bash
kubectl apply -f sim_v1alpha1_simulatordeployment_istio.yaml
```

### Check Status

```bash
# Watch SimulatorDeployment
kubectl get simulatordeployment -n llm-d-sim -w

# Check created resources
kubectl get all -n llm-d-sim

# View detailed status
kubectl describe simulatordeployment -n llm-d-sim
```

## Sample Comparison

| Feature | Minimal | Istio |
|---------|---------|-------|
| Replicas | 2 | 2 |
| Image | Default | Explicit |
| Service Type | ClusterIP | ClusterIP |
| Gateway | Disabled | Enabled (Istio) |
| Load Balancing | Default | Round-Robin |
| Connection Pool | Default | Custom |

## Customization Examples

### Change Replica Count

```yaml
spec:
  replicas: 4
```

### Use Custom Image

```yaml
spec:
  image: "your-registry/llm-d-simulator:v1.0.0"
```

### Configure Resources

```yaml
spec:
  resources:
    requests:
      cpu: "4"
      memory: "8Gi"
    limits:
      cpu: "8"
      memory: "16Gi"
```

### Change Service Type

```yaml
spec:
  service:
    type: "LoadBalancer"
```

### Enable Load Balancing

```yaml
spec:
  loadBalancing:
    enabled: true
    algorithm: "LEAST_REQUEST"  # or ROUND_ROBIN, RANDOM, LEAST_CONN
```

## Integration with llm-d-scheduler

These samples create resources compatible with the llm-d scheduler:

1. **Deployment Name:** `ms-sim-{name}-decode`
2. **Pod Labels:** `llm-d.ai/role=decode`, `llm-d.ai/inferenceServing=true`
3. **Service Name:** `gaie-inference-scheduling-proxy`
4. **Service Port:** 8200

### Testing with Scheduler

```bash
# Set environment
export NAMESPACE_IS=llm-d-inference-scheduler
export NAMESPACE_SIM=llm-d-sim

# Get scheduler route
SCHED_ROUTE_URL=$(oc get route llm-sched -n ${NAMESPACE_IS} -o jsonpath='{.spec.host}')

# Send test requests
for i in {1..8}; do
  curl -X POST http://${SCHED_ROUTE_URL}/v1/completions \
    -H "Content-Type: application/json" \
    -d '{"model": "random", "prompt": "test", "max_tokens": 5}'
  sleep 0.5
done
```

## Troubleshooting

### Pods Not Starting

```bash
# Check events
kubectl describe simulatordeployment -n llm-d-sim

# Check pod logs
kubectl logs -n llm-d-sim -l llm-d.ai/role=decode
```

### Service Not Working

```bash
# Check service
kubectl get service gaie-inference-scheduling-proxy -n llm-d-sim

# Check endpoints
kubectl get endpoints gaie-inference-scheduling-proxy -n llm-d-sim
```

### Load Balancing Issues

```bash
# Verify Istio is installed
kubectl get pods -n istio-system

# Check DestinationRule (if load balancing enabled)
kubectl get destinationrule -n llm-d-sim
```

## Related Documentation

- [Main README](../../README.md)
- [llm-d-scheduler-sim Manual Setup](https://github.com/llm-d/llm-d-scheduler-sim/blob/main/hands-on/2_simulators.md)
- [Scheduler-Simulator Integration](https://github.com/llm-d/llm-d-scheduler-sim/blob/main/hands-on/sched-sim.md)