# Manual EnvoyFilter ext_proc PoC Guide

This guide provides a step-by-step manual approach to set up and test the Istio `ext_proc` filter integration with the EPP (Endpoint Picker) service. This is useful for quick prototyping and debugging before implementing operator-managed automation.

## Prerequisites

- Minikube cluster running with IPVS mode
- Istio installed with gateway API support
- `SchedulerInstall` and `SimulatorDeployment` resources deployed
- Gateway and EPP services running in `llm-d-inference-scheduler` namespace

## Step 1: Verify Prerequisites

### Check that the EPP service exists and has a ClusterIP

```bash
kubectl get svc -n llm-d-inference-scheduler gaie-inference-scheduling-epp
```

Expected output should show a ClusterIP (e.g., `10.96.x.x`) and port `9002`.

### Check that the gateway pod is running

```bash
kubectl get pods -n llm-d-inference-scheduler \
  -l gateway.networking.k8s.io/gateway-name=infra-inference-scheduling-inference-gateway
```

### Verify the gateway service

```bash
kubectl get svc -n llm-d-inference-scheduler \
  -l gateway.networking.k8s.io/gateway-name=infra-inference-scheduling-inference-gateway
```

Note the ClusterIP for later testing.

## Step 2: Create the EnvoyFilter Manifest

Based on lessons from production debugging, create an improved version of the EnvoyFilter:

```bash
cat <<'EOF' > /tmp/envoyfilter-epp-poc.yaml
apiVersion: networking.istio.io/v1alpha3
kind: EnvoyFilter
metadata:
  name: epp-ext-proc-poc
  namespace: llm-d-inference-scheduler
spec:
  workloadSelector:
    labels:
      gateway.networking.k8s.io/gateway-name: infra-inference-scheduling-inference-gateway
  configPatches:
  # Patch 1: Configure the ext_proc HTTP filter
  - applyTo: HTTP_FILTER
    match:
      context: GATEWAY
      listener:
        filterChain:
          filter:
            name: envoy.filters.network.http_connection_manager
            subFilter:
              name: envoy.filters.http.ext_proc
    patch:
      operation: REPLACE
      value:
        name: envoy.filters.http.ext_proc
        typed_config:
          "@type": type.googleapis.com/envoy.extensions.filters.http.ext_proc.v3.ExternalProcessor
          grpc_service:
            envoy_grpc:
              cluster_name: epp-cluster
            timeout: 2s
          processing_mode:
            request_header_mode: SKIP
            request_body_mode: NONE
            response_header_mode: SKIP
            response_body_mode: NONE
  
  # Patch 2: Add the EPP cluster configuration
  - applyTo: CLUSTER
    match:
      context: GATEWAY
    patch:
      operation: ADD
      value:
        name: epp-cluster
        type: STATIC
        connect_timeout: 2s
        lb_policy: ROUND_ROBIN
        http2_protocol_options: {}
        load_assignment:
          cluster_name: epp-cluster
          endpoints:
          - lb_endpoints:
            - endpoint:
                address:
                  socket_address:
                    address: EPP_SERVICE_CLUSTER_IP  # Replace this!
                    port_value: 9002
EOF
```

### Replace the placeholder with actual ClusterIP

```bash
EPP_CLUSTER_IP=$(kubectl get svc -n llm-d-inference-scheduler \
  gaie-inference-scheduling-epp -o jsonpath='{.spec.clusterIP}')

echo "EPP Service ClusterIP: $EPP_CLUSTER_IP"

sed -i.bak "s/EPP_SERVICE_CLUSTER_IP/$EPP_CLUSTER_IP/g" /tmp/envoyfilter-epp-poc.yaml

# Verify the replacement
grep "address:" /tmp/envoyfilter-epp-poc.yaml | tail -1
```

## Step 3: Apply the EnvoyFilter

```bash
kubectl apply -f /tmp/envoyfilter-epp-poc.yaml
```

Verify it was created:
```bash
kubectl get envoyfilter -n llm-d-inference-scheduler epp-ext-proc-poc
```

## Step 4: Wait for Configuration to Propagate

Envoy needs time to receive and apply the new configuration from Istiod:

```bash
echo "Waiting 10 seconds for Envoy config to propagate..."
sleep 10
```

## Step 5: Verify Envoy Configuration

### Check that the ext_proc filter is configured

```bash
GATEWAY_POD=$(kubectl get pods -n llm-d-inference-scheduler \
  -l gateway.networking.k8s.io/gateway-name=infra-inference-scheduling-inference-gateway \
  -o jsonpath='{.items[0].metadata.name}')

echo "Gateway pod: $GATEWAY_POD"

# Check for ext_proc in the listener configuration
kubectl exec -n llm-d-inference-scheduler $GATEWAY_POD -- \
  pilot-agent request GET config_dump | grep -A 10 "ext_proc"
```

### Check that the epp-cluster exists

```bash
kubectl exec -n llm-d-inference-scheduler $GATEWAY_POD -- \
  pilot-agent request GET clusters | grep "epp-cluster"
```

Expected output should show:
- `epp-cluster::observability_name::epp-cluster`
- Various statistics like `cx_total`, `rq_total`, etc.

## Step 6: Add Per-Route Override (Critical!)

The global filter is set to `SKIP` mode. We need a per-route override to activate it for specific routes.

First, check your existing HTTPRoute:

```bash
kubectl get httproute -n llm-d-inference-scheduler -o yaml
```

Create a patch to add the ext_proc override:

```bash
cat <<'EOF' > /tmp/httproute-extproc-patch.yaml
spec:
  rules:
  - filters:
    - type: ExtensionRef
      extensionRef:
        group: networking.istio.io
        kind: EnvoyFilter
        name: epp-ext-proc-poc
    backendRefs:
    - group: inference.networking.k8s.io
      kind: InferencePool
      name: gaie-inference-scheduling
      namespace: llm-d-sim
EOF
```

**Note:** You'll need to merge this with your existing HTTPRoute configuration. The exact patch depends on your current route structure.

Alternatively, add the override using Istio's VirtualService annotation or modify the HTTPRoute to include:

```yaml
metadata:
  annotations:
    networking.istio.io/ext-proc-override: |
      request_header_mode: SEND
      request_body_mode: NONE
```

## Step 7: Send Test Traffic

### Get the gateway ClusterIP

```bash
GATEWAY_IP=$(kubectl get svc -n llm-d-inference-scheduler \
  -l gateway.networking.k8s.io/gateway-name=infra-inference-scheduling-inference-gateway \
  -o jsonpath='{.spec.clusterIP}')

echo "Gateway ClusterIP: $GATEWAY_IP"
```

### Send a test request from within the cluster

```bash
kubectl run -n llm-d-inference-scheduler curl-test --rm -i --restart=Never \
  --image=curlimages/curl -- \
  -v -X POST http://$GATEWAY_IP/v1/completions \
  -H "Content-Type: application/json" \
  -d '{"model": "random", "prompt": "test", "max_tokens": 5}'
```

### Alternative: Send from the gateway pod itself

```bash
kubectl exec -n llm-d-inference-scheduler $GATEWAY_POD -- \
  curl -v -X POST http://localhost:80/v1/completions \
  -H "Content-Type: application/json" \
  -d '{"model": "random", "prompt": "test", "max_tokens": 5}'
```

## Step 8: Verify ext_proc Invocation

### Check EPP logs for processing

```bash
EPP_POD=$(kubectl get pods -n llm-d-inference-scheduler \
  -l app=gaie-inference-scheduling-epp \
  -o jsonpath='{.items[0].metadata.name}')

echo "EPP pod: $EPP_POD"

kubectl logs -n llm-d-inference-scheduler $EPP_POD --tail=100
```

Look for log entries indicating request processing.

### Check epp-cluster statistics

```bash
kubectl exec -n llm-d-inference-scheduler $GATEWAY_POD -- \
  pilot-agent request GET clusters | grep "epp-cluster" | grep -E "rq_total|cx_total|rq_success"
```

After sending traffic, `rq_total` should increment.

### Check for errors in gateway logs

```bash
kubectl logs -n llm-d-inference-scheduler $GATEWAY_POD --tail=100 | grep -i "ext_proc\|epp"
```

## Step 9: Debugging Common Issues

### Issue 1: ext_proc not invoked (rq_total stays at 0)

**Possible causes:**
- Per-route override not configured
- Route not matching the request path
- Filter order incorrect

**Debug:**
```bash
# Check the full listener config
kubectl exec -n llm-d-inference-scheduler $GATEWAY_POD -- \
  pilot-agent request GET config_dump > /tmp/envoy-config.json

# Search for ext_proc configuration
grep -A 50 "ext_proc" /tmp/envoy-config.json
```

### Issue 2: Connection failures to EPP

**Possible causes:**
- Wrong ClusterIP
- EPP service not ready
- Port mismatch

**Debug:**
```bash
# Test direct connectivity from gateway to EPP
kubectl exec -n llm-d-inference-scheduler $GATEWAY_POD -- \
  nc -zv $EPP_CLUSTER_IP 9002

# Check EPP service endpoints
kubectl get endpoints -n llm-d-inference-scheduler gaie-inference-scheduling-epp
```

### Issue 3: Requests blocked or timing out

**Possible causes:**
- `request_body_mode` set to `BUFFERED` or `STREAMED`
- EPP not responding to gRPC calls

**Fix:**
Ensure `request_body_mode: NONE` in the EnvoyFilter.

## Step 10: Cleanup

To remove the manual PoC configuration:

```bash
kubectl delete envoyfilter -n llm-d-inference-scheduler epp-ext-proc-poc
rm /tmp/envoyfilter-epp-poc.yaml /tmp/envoyfilter-epp-poc.yaml.bak
```

## Key Differences from Production Approach

This manual PoC differs from the operator-managed approach in `ext-proc-fixes.md`:

1. **Manual ClusterIP substitution** - You must manually fetch and replace the EPP ClusterIP
2. **No reconciliation** - Changes can drift if resources are recreated
3. **Manual per-route override** - Requires manual HTTPRoute modification
4. **Simplified for testing** - Uses `REPLACE` operation but may conflict with existing filters

## Next Steps

Once the PoC is validated:
1. Implement the configuration in the operator's `SchedulerInstall` controller
2. Add dynamic ClusterIP fetching
3. Implement proper reconciliation logic
4. Add per-route override automation

## References

- Production fixes: `doc/ext-proc-fixes.md`
- Original sample: `config/samples/envoyfilter-epp.yaml`
- Envoy ext_proc docs: https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_filters/ext_proc_filter
