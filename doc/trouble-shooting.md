# Troubleshooting

### Pods CrashLoopBackOff

1.  **Check Probes**:
    -   **Gateway**: Ensure readiness/liveness probes use port `19000` (Admin interface), NOT `8082` or `8080`.
    -   **EPP**: Ensure probes use TCP socket on port `9003`. gRPC probes may fail if the service name doesn't match.

2.  **Check Permissions (RBAC)**:
    -   If EPP crashes with "forbidden" errors, run `./hack/create-epp-rbac.sh` to fix ServiceAccount permissions.

3.  **Check Logs**:
    ```bash
    kubectl logs -n llm-d-sim -l component=epp
    kubectl logs -n llm-d-sim -l component=gateway
    ```

### Service Endpoints Not Populated / Connection Refused

1.  **Check Service TargetPort**:
    -   The Gateway Service must target port `80` (where the pod listens), not `8080`.
    -   Verify with: `kubectl get svc infra-sim-inference-gateway -n llm-d-sim -o yaml`

2.  **Check Backend Configuration**:
    -   Ensure Gateway ConfigMap points to the correct backend service (e.g., `ms-sim-llm-d-modelservice-decode`) on port `8200`.
    -   "No healthy upstream" usually means the Envoy config points to a wrong service or port.

### Load Balancing Not Working

```bash
# Check if DestinationRule was created (requires Istio)
kubectl get destinationrule -n llm-d-sim

# Verify Istio is installed
kubectl get pods -n istio-system
```

### Gateway API Resources Missing

If `kubectl get gateway,httproute` returns nothing or the CRDs are missing:

```bash
kubectl apply -k ../llm-d-inference-scheduler/deploy/components/crds-gateway-api
kubectl api-resources | rg -i "gateway|httproute|referencegrant"
```

### Gateway Programmed False

If the Gateway is not programmed, confirm the GatewayClass exists and matches your configuration:

```bash
kubectl get gatewayclass
kubectl describe gateway -n llm-d-inference-scheduler infra-inference-scheduling-inference-gateway
```

The operator default is `istio`. If your cluster only has `kgateway`, update the SchedulerInstall spec to use that class.

### No Healthy Upstream (Gateway)

Common cause is missing endpoints for the proxy Service or miswired HTTPRoute/ReferenceGrant.

```bash
kubectl get endpointslice -n llm-d-sim \
  -l kubernetes.io/service-name=gaie-inference-scheduling-proxy
kubectl get httproute -n llm-d-inference-scheduler -o yaml
kubectl get referencegrant -n llm-d-sim -o yaml
```

Also ensure the request is going through the scheduler gateway Service:
`infra-inference-scheduling-inference-gateway-istio` in `llm-d-inference-scheduler`.

### EndpointSlice vs Endpoints Warning

`kubectl get endpoints` warns on newer clusters. Use EndpointSlice instead:

```bash
kubectl get endpointslice -n llm-d-sim \
  -l kubernetes.io/service-name=ms-sim-llm-d-modelservice-decode
```

### Port-Forward Conflicts

If local port 8080 is in use, pick a different local port:

```bash
kubectl port-forward -n llm-d-inference-scheduler \
  svc/infra-inference-scheduling-inference-gateway-istio 18080:80
```

### Scheduler Scoring Not Applied

EPP scoring only works when the Gateway is configured with ext_proc (EnvoyFilter on Istio).
See `doc/future-work.md` for the EnvoyFilter sample.

### Bypass vs InferencePool Routing

The current workflow uses a proxy Service bypass. This is reliable for routing, but it bypasses
InferencePool selection, so KV-scoring tests won’t be meaningful unless you route via InferencePool.
