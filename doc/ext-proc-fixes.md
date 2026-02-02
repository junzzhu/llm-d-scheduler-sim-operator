# ext_proc Filter Configuration Fixes

This document describes the process of identifying and resolving issues where the Istio Envoy `ext_proc` filter was not correctly invoking the EPP (Endpoint Picker) service, ultimately leading to a permanent fix in the operator's controller code.

## Background

The LLM-D inference scheduler uses Istio's `ext_proc` filter to intercept HTTP requests at the gateway and delegate endpoint selection to the EPP service via gRPC. Several configuration issues initially prevented `ext_proc` from being invoked or from functioning correctly.

## Issues Identified

1.  **EnvoyFilter Reconciliation:** The primary issue was that the `EnvoyFilter` created by the operator's `SchedulerInstall` controller had incorrect configurations, and any manual patches to fix this were being reverted by the controller's reconciliation loop.
2.  **Wrong EnvoyFilter Operation**: The `EnvoyFilter` used `INSERT_BEFORE` relative to the `router` filter instead of `REPLACE` on the `envoy.filters.http.ext_proc` filter itself, leading to potential conflicts or improper application.
3.  **Request Body Mode**: `STREAMED` mode was blocking processing of requests.
4.  **DNS Resolution Failure**: The Gateway pod was unable to reliably resolve the EPP service DNS name, leading to connection failures.
5.  **Global Filter Mode Conflict**: The global `request_header_mode` was `SEND`, which conflicted with the desired behavior of using per-route overrides effectively.

## Fixes Applied

The permanent solution involved modifying the `reconcileEnvoyFilter` function within the `controllers/schedulerinstall_controller.go` to ensure the `EnvoyFilter` is created with the correct and persistent configuration.

### Fix 1: Updated EnvoyFilter Operation to REPLACE and Correct Matching

The controller code was updated to change the EnvoyFilter operation from `INSERT_BEFORE` to `REPLACE`. It now correctly matches on `envoy.filters.http.ext_proc` to ensure proper configuration of the filter.

### Fix 2: Changed Request Body Mode to NONE

The `request_body_mode` was changed from `STREAMED` to `NONE` in both the global filter configuration and the per-route override within the controller code. This prevents the filter from blocking on the request body.

### Fix 3: Changed Cluster Type to STATIC with Dynamic ClusterIP

To address DNS resolution issues, the controller was updated to:
*   Dynamically fetch the `ClusterIP` of the EPP service (`gaie-inference-scheduling-epp`).
*   Configure the `epp-cluster` with `type: "STATIC"` and use the fetched `ClusterIP` directly.

This ensures reliable connectivity to the EPP service.

### Fix 4: Set Global Filter to SKIP Mode with Per-Route Override to SEND

The global `request_header_mode` in the main `ext_proc` filter configuration was set to `SKIP`. The per-route override `request_header_mode` was explicitly set to `SEND`. This ensures that headers are sent to the EPP service when the specific route is matched, while allowing the global filter to be in a passthrough mode when no override is active.

### Fix 5: Operator Restart Required for Permanent Changes

After applying the code changes to `controllers/schedulerinstall_controller.go`, the operator managing the `SchedulerInstall` custom resource must be rebuilt and restarted. This ensures that the controller reconciles the `EnvoyFilter` with the corrected configuration, making the fixes permanent.

## Current Status

### Configuration Verified ✅

The EnvoyFilter, once correctly configured by the updated operator:
-   HTTP filter chain correctly includes `ext_proc` matching on `envoy.filters.http.ext_proc` with `operation: REPLACE`.
-   `ext_proc` global configuration has `request_header_mode: "SKIP"` and `request_body_mode: "NONE"`.
-   `epp-cluster` correctly uses `type: "STATIC"` with the EPP service's ClusterIP.
-   Per-route override correctly sets `request_header_mode: "SEND"` and `request_body_mode: "NONE"`.
-   No errors observed in gateway or EPP logs related to configuration or connection.

### ext_proc Invocation Status ⚠️

Despite the correct configuration in Envoy and `epp-cluster` statistics showing incrementing request totals (`rq_total`), the EPP logs (even at verbosity 9) do not show any processing messages for incoming requests. This indicates that while Envoy is successfully sending requests to the EPP service, the EPP service is not processing or logging them as expected. This suggests a potential mismatch in the gRPC communication or an issue within the EPP service's request handling.

## Verification Commands

### Check Envoy Cluster Connectivity and Request Count

To verify that requests are reaching the EPP cluster:
```bash
GATEWAY_POD=$(kubectl get pods -n llm-d-inference-scheduler \
  -l gateway.networking.k8s.io/gateway-name=infra-inference-scheduling-inference-gateway \
  -o jsonpath='{.items[0].metadata.name}')

kubectl exec -n llm-d-inference-scheduler $GATEWAY_POD -- \
  pilot-agent request GET clusters | grep "epp-cluster"
```
Look for `cx_total` and `rq_total` to be greater than 0 after sending test traffic.

### Check EPP Logs for Processing Details

To confirm the EPP service is processing the requests:
```bash
EPP_POD=$(kubectl get pods -n llm-d-inference-scheduler \
  -l app=gaie-inference-scheduling-epp \
  -o jsonpath='{.items[0].metadata.name}')

kubectl logs -n llm-d-inference-scheduler $EPP_POD --tail=50
```
(Ensure EPP verbosity is set to 9 in `SchedulerInstall` for detailed logs. Currently, logs show only startup messages, not processing of requests.)

### Send Test Request

To generate traffic and trigger `ext_proc` invocation:
```bash
GATEWAY_POD=$(kubectl get pods -n llm-d-inference-scheduler \
  -l gateway.networking.k8s.io/gateway-name=infra-inference-scheduling-inference-gateway \
  -o jsonpath='{.items[0].metadata.name}')

kubectl exec -n llm-d-inference-scheduler $GATEWAY_POD -- \
  curl -s -X POST http://localhost:80/v1/completions \
  -H "Content-Type: application/json" \
  -d '{"model": "random", "prompt": "test", "max_tokens": 5}'
```

## Next Steps

With the `ext_proc` filter correctly configured and Envoy sending requests to EPP, the next step would be to investigate why the EPP service is not logging these requests despite high verbosity. This would likely involve debugging the EPP service itself, potentially examining its gRPC server implementation to ensure it correctly handles incoming `envoy.service.ext_proc.v3.ExternalProcessor` requests.