# EPP Prefix Cache Scorer Test Results Summary

## Test Overview
- **Test Script**: `kv-prefix-cache-test.sh` (enhanced version)
- **EPP Pod**: gaie-inference-scheduling-epp-65d9d6ff9c-rw528
- **Number of Test Iterations**: 8
- **Test Prompt**: 5000+ character string to ensure significant prefix cache

## Test Methodology
1. Send first request through gateway with identical prompt (cold cache)
2. Send 7 additional requests with same prompt
3. Monitor EPP scoring decisions via logs
4. Verify prefix-cache-scorer correctly identifies cached prefixes

## Test Results

### Test Run (18:38:47 - 18:39:25)

**Request 1** (18:38:47):
- All 3 pods scored 0.000 (cold cache state)
- Pod `t9z9v` selected randomly
- Weighted scores: All pods = 1.500

**Requests 2-8** (18:38:53 - 18:39:25):
- Pod `t9z9v` consistently scored 1.000 (100% cache hit)
- Other pods (`rvmbl`, `8ds4c`) scored 0.000 (no cache)
- Weighted scores: t9z9v = 3.500, others = 1.500

### Scoring Breakdown Analysis

For each request after the first:
- **Load-aware scorer**: 0.500 (all pods equal) × 1.0 weight = 0.500
- **Prefix-cache scorer**: 1.000 (cached pod) vs 0.000 (others) × 2.0 weight = 2.000 vs 0.000
- **KV-cache-utilization scorer**: 1.000 (all pods) × 1.0 weight = 1.000
- **Total weighted score**: 3.500 (cached) vs 1.500 (non-cached)

### Pod Selection Summary

| Request # | Timestamp | Selected Pod | Prefix Score | Total Score |
|-----------|-----------|--------------|--------------|-------------|
| 1 | 18:38:47 | t9z9v | 0.000 | 1.500 |
| 2 | 18:38:53 | t9z9v | 1.000 | 3.500 |
| 3 | 18:38:59 | t9z9v | 1.000 | 3.500 |
| 4 | 18:39:05 | t9z9v | 1.000 | 3.500 |
| 5 | 18:39:09 | t9z9v | 1.000 | 3.500 |
| 6 | 18:39:14 | t9z9v | 1.000 | 3.500 |
| 7 | 18:39:20 | t9z9v | 1.000 | 3.500 |
| 8 | 18:39:25 | t9z9v | 1.000 | 3.500 |

## Key Findings

### ✅ Verified Behaviors

1. **Cold Cache Detection**: First request correctly shows all pods with 0.000 prefix score
2. **Cache Hit Recognition**: Subsequent requests correctly identify the pod with cached prefix (1.000 score)
3. **Consistent Routing**: All requests after the first are routed to the same pod (t9z9v)
4. **Scoring Weight**: Prefix-cache scorer's 2x weight correctly influences final routing decision
5. **Score Calculation**: Weighted total of 3.500 (cached) vs 1.500 (non-cached) demonstrates proper scoring

### 🔍 Technical Details

**Prefix Cache Scorer**:
- Location: `_deps/gateway-api-inference-extension/pkg/epp/scheduling/framework/plugins/multi/prefix/plugin.go`
- Score calculation: `matchLen / total` where matchLen is number of matching prefix blocks
- Weight multiplier: 2.0x (configured in scheduler profile)
- Indexer: LRU-based with default capacity of 31,250 blocks per pod

**PreRequest Hook**:
- Updates indexer after each routing decision
- Maintains `hashToPods` map for quick lookup
- Ensures cache state is synchronized with actual pod state

## Previous Issue Resolution

### Root Cause Identified
The original test issue was caused by **stale indexer state** from a long-running EPP pod (14+ hours uptime). Previous test runs had populated the indexer, causing incorrect scoring on subsequent tests.

### Solution Applied
Restart EPP pod before running tests to ensure clean indexer state:
```bash
kubectl delete pod -n llm-d-inference-scheduler -l app.kubernetes.io/name=gaie-inference-scheduling-epp
```

## Recommendations

### For Testing
1. Always restart EPP pod before running prefix cache tests
2. Use the enhanced test script which validates JSON responses
3. Run at least 8 iterations to verify consistency
4. Monitor EPP logs for scoring breakdown details

### For Production
1. Consider implementing indexer TTL or periodic cleanup
2. Monitor EPP pod uptime and restart periodically if needed
3. Add metrics for prefix cache hit rates
4. Consider exposing indexer state via debug endpoints

## Conclusion

**Status**: ✅ **VERIFIED - Working Correctly**

The EPP prefix-cache-scorer is functioning as designed:
- Correctly identifies cold vs warm cache states
- Properly calculates prefix match scores
- Successfully influences routing decisions via 2x weight multiplier
- Maintains consistent behavior across multiple test iterations

The implementation correctly leverages prefix caching to optimize request routing, reducing latency by directing requests with matching prefixes to pods that already have the relevant data cached.

## Test Script Location
- Main test script: `llm-d-scheduler-sim-operator/script/kv-prefix-cache-test.sh`
