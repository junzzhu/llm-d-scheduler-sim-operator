# Future Work

## EnvoyFilter (Istio ext_proc -> EPP)

This enables scoring by wiring the Gateway to the EPP service via ext_proc.

Sample manifest:
`config/samples/envoyfilter-epp.yaml`

Apply:
```bash
kubectl apply -f config/samples/envoyfilter-epp.yaml
```

Validation:
```bash
kubectl get envoyfilter -n llm-d-inference-scheduler

istioctl proxy-config listeners -n llm-d-inference-scheduler \
  $(kubectl get pod -n llm-d-inference-scheduler \
    -l gateway.networking.k8s.io/gateway-name=infra-inference-scheduling-inference-gateway \
    -o jsonpath='{.items[0].metadata.name}') | rg ext_proc

kubectl logs -n llm-d-inference-scheduler \
  deploy/gaie-inference-scheduling-epp --tail=100
```
