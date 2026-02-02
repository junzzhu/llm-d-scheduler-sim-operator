#!/bin/bash

GATEWAY_IP="10.97.89.229"
PROMPT=$(python3 -c "print('verify-prefix-test-' + 'x'*5000)")
NUM_TESTS=8

# Get EPP pod name using grep instead of label selector
EPP_POD=$(kubectl get pod -n llm-d-inference-scheduler | grep "gaie-inference-scheduling-epp" | grep "Running" | awk '{print $1}')

if [ -z "$EPP_POD" ]; then
    echo "ERROR: Could not find running EPP pod"
    exit 1
fi

echo "Using EPP Pod: $EPP_POD"
echo "Running $NUM_TESTS test requests with same prompt"
echo "=================================================="
echo ""

for i in $(seq 1 $NUM_TESTS); do
    echo "=== Test $i: Sending request ==="
    
    # Send request and capture response
    RESPONSE=$(kubectl run curl-test$i -n llm-d-inference-scheduler --image=curlimages/curl:latest --rm -i --restart=Never -- \
      curl -s -X POST http://$GATEWAY_IP/v1/completions \
      -H "Content-Type: application/json" \
      -d '{"model":"random","prompt":"'"$PROMPT"'","max_tokens":32}' 2>/dev/null)
    
    # Try to extract ID, handle both success and error cases
    if echo "$RESPONSE" | jq -e '.id' > /dev/null 2>&1; then
        echo "$RESPONSE" | jq -r '.id'
    else
        echo "Request completed (response may not be valid JSON)"
    fi
    
    sleep 2
    
    # Get scoring from EPP logs
    echo "Scoring:"
    kubectl logs -n llm-d-inference-scheduler $EPP_POD --tail=20 | \
      grep "prefix-cache-scorer\[" | tail -1 | \
      sed 's/.*prefix-cache-scorer\[\([^]]*\)\].*/  \1/'
    
    # Get routing decision
    echo "Selected pod:"
    kubectl logs -n llm-d-inference-scheduler $EPP_POD --tail=10 | \
      grep "Scoring decision" | tail -1 | \
      sed 's/.*"selected":\["\([^"]*\)".*/  \1/' | \
      sed 's/.*-\([^-]*\)-rank-0/  \1/'
    
    echo ""
done

echo "=================================================="
echo "=== Analysis ==="
echo ""
echo "Full prefix-cache-scorer logs:"
kubectl logs -n llm-d-inference-scheduler $EPP_POD | grep "prefix-cache-scorer\[" | nl

echo ""
echo "Expected behavior:"
echo "  Request 1: Both pods score 0.000 (cold cache, random selection)"
echo "  Request 2-8: Selected pod from request 1 scores 1.000, other scores 0.000"
echo ""
echo "Actual pod selections:"
kubectl logs -n llm-d-inference-scheduler $EPP_POD | \
  grep "Scoring decision" | \
  sed 's/.*"selected":\["\([^"]*\)".*/\1/' | \
  sed 's/.*-\([^-]*\)-rank-0/\1/' | \
  nl
