# Troubleshooting Guide

Common issues and their solutions when using the Vector Sidecar Operator.

## Table of Contents

- [Quick Diagnostics](#quick-diagnostics)
- [Common Issues](#common-issues)
- [Debugging Steps](#debugging-steps)
- [Log Analysis](#log-analysis)
- [Performance Issues](#performance-issues)

## Quick Diagnostics

Run these commands to get an overview of your setup:

```bash
# Check operator status
kubectl get deployment -n vector-sidecar-operator-system

# Check all VectorSidecars
kubectl get vectorsidecar -A

# Check CRD installation
kubectl get crd vectorsidecars.observability.kontroloop.ai

# View operator logs
kubectl logs -n vector-sidecar-operator-system deployment/vector-sidecar-operator-controller-manager --tail=50
```

## Common Issues

### Issue 1: Sidecar Not Injected

**Symptoms:**
- VectorSidecar shows `matchedDeployments: 0`
- Deployment pods don't have Vector container
- No errors in operator logs

**Diagnosis:**

```bash
# Check VectorSidecar status
kubectl describe vectorsidecar <name>

# Check deployment labels
kubectl get deployment <name> --show-labels

# List all deployments with specific label
kubectl get deployments -l <label-key>=<label-value>
```

**Common Causes:**

1. **Label Mismatch**

```bash
# VectorSidecar selector
spec:
  selector:
    matchLabels:
      observability: vector

# But deployment has
labels:
  observability: enabled  # Wrong value!
```

**Solution:** Fix deployment labels or VectorSidecar selector

```bash
kubectl label deployment <name> observability=vector --overwrite
```

2. **Wrong Namespace**

VectorSidecar only matches deployments in the same namespace.

**Solution:** Create VectorSidecar in the same namespace as your deployments

```bash
kubectl apply -f vectorsidecar.yaml -n <target-namespace>
```

3. **Deployment Doesn't Exist Yet**

**Solution:** Create deployment after VectorSidecar, or the operator will inject on next reconciliation

---

### Issue 2: ConfigMap Not Found

**Symptoms:**
- Status shows `ConfigValid: False`
- Error message: "ConfigMap not found"
- Matched deployments but not injected

**Diagnosis:**

```bash
# Check VectorSidecar status
kubectl get vectorsidecar <name> -o jsonpath='{.status.conditions}'

# Check if ConfigMap exists
kubectl get configmap <configmap-name> -n <namespace>
```

**Solution:**

```bash
# Create the ConfigMap first
kubectl create configmap vector-config \
  --from-file=vector.yaml=./vector-config.yaml \
  -n <namespace>

# Or use kubectl apply
kubectl apply -f - <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: vector-config
  namespace: default
data:
  vector.yaml: |
    sources:
      kubernetes_logs:
        type: kubernetes_logs
    sinks:
      stdout:
        type: console
        inputs: [kubernetes_logs]
EOF
```

---

### Issue 3: Operator Not Running

**Symptoms:**
- VectorSidecar resources don't reconcile
- No status updates
- `kubectl get vectorsidecar` shows no status changes

**Diagnosis:**

```bash
# Check operator pod status
kubectl get pods -n vector-sidecar-operator-system

# Check operator logs
kubectl logs -n vector-sidecar-operator-system \
  deployment/vector-sidecar-operator-controller-manager
```

**Common Causes:**

1. **CRDs Not Installed**

```bash
# Check if CRD exists
kubectl get crd vectorsidecars.observability.kontroloop.ai
```

**Solution:** Install CRDs

```bash
make install
# or
kubectl apply -f config/crd/bases/
```

2. **RBAC Permissions Missing**

**Solution:** Verify operator has required permissions

```bash
kubectl describe serviceaccount -n vector-sidecar-operator-system
kubectl get clusterrolebinding | grep vector-sidecar
```

3. **Image Pull Error**

```bash
kubectl describe pod -n vector-sidecar-operator-system
```

**Solution:** Check image availability and pull secrets

---

### Issue 4: Vector Container CrashLooping

**Symptoms:**
- Pod restarts frequently
- Vector container shows `CrashLoopBackOff`
- Application container works fine

**Diagnosis:**

```bash
# Check pod status
kubectl get pod <pod-name> -o jsonpath='{.status.containerStatuses[?(@.name=="vector")]}'

# Check Vector logs
kubectl logs <pod-name> -c vector --previous

# Check events
kubectl describe pod <pod-name>
```

**Common Causes:**

1. **Invalid Vector Configuration**

```bash
# Check Vector config
kubectl logs <pod-name> -c vector --previous | grep -i error
```

**Solution:** Validate Vector configuration

```bash
# Test config locally
docker run --rm -v $(pwd)/vector.yaml:/etc/vector/vector.yaml \
  timberio/vector:0.35.0 validate /etc/vector/vector.yaml
```

2. **Resource Limits Too Low (OOMKilled)**

```bash
# Check for OOM kills
kubectl get pod <pod-name> -o jsonpath='{.status.containerStatuses[?(@.name=="vector")].lastState.terminated.reason}'
```

**Solution:** Increase memory limits

```yaml
sidecar:
  resources:
    limits:
      memory: 512Mi  # Increase from 256Mi
```

3. **Missing Volume Mounts**

Vector can't access required paths.

**Solution:** Add necessary volumes and mounts

```yaml
sidecar:
  volumeMounts:
    - name: varlog
      mountPath: /var/log
volumes:
  - name: varlog
    emptyDir: {}
```

---

### Issue 5: Logs Not Appearing in Sink

**Symptoms:**
- Vector running without errors
- No logs reaching destination (Elasticsearch, S3, etc.)
- No connection errors in Vector logs

**Diagnosis:**

```bash
# Enable debug logging
kubectl set env deployment/<name> VECTOR_LOG=debug -c vector

# Watch Vector logs
kubectl logs <pod-name> -c vector -f

# Check Vector internal metrics
kubectl port-forward <pod-name> 9598:9598
curl localhost:9598/metrics | grep component_errors_total
```

**Common Causes:**

1. **Network Policy Blocking Traffic**

```bash
# Check network policies
kubectl get networkpolicy -n <namespace>

# Test connectivity from pod
kubectl exec <pod-name> -c vector -- curl -v <sink-endpoint>
```

**Solution:** Update NetworkPolicy to allow egress

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-vector-egress
spec:
  podSelector:
    matchLabels:
      app: your-app
  policyTypes:
    - Egress
  egress:
    - to:
        - podSelector:
            matchLabels:
              app: elasticsearch
      ports:
        - protocol: TCP
          port: 9200
```

2. **Incorrect Sink Configuration**

**Solution:** Verify sink endpoint and credentials

```yaml
sinks:
  elasticsearch:
    type: elasticsearch
    endpoint: "https://elasticsearch:9200"  # Verify endpoint
    auth:
      strategy: basic
      user: "${ELASTICSEARCH_USER}"
      password: "${ELASTICSEARCH_PASSWORD}"
```

3. **Rate Limiting or Backpressure**

Vector is being throttled by the sink.

**Solution:** Add buffering and retry configuration

```yaml
sinks:
  elasticsearch:
    type: elasticsearch
    inputs: [transform]
    endpoint: "https://elasticsearch:9200"
    batch:
      max_bytes: 10485760
      timeout_secs: 60
    buffer:
      type: disk
      max_size: 268435488  # 256MB
    request:
      retry_attempts: 5
      retry_initial_backoff_secs: 1
```

---

### Issue 6: High Resource Usage

**Symptoms:**
- Vector using too much CPU or memory
- Application performance degraded
- Cluster resource pressure

**Diagnosis:**

```bash
# Check current resource usage
kubectl top pod <pod-name> --containers

# Check resource limits
kubectl get pod <pod-name> -o jsonpath='{.spec.containers[?(@.name=="vector")].resources}'

# Monitor over time
watch -n 5 'kubectl top pod <pod-name> --containers'
```

**Solutions:**

1. **Reduce Log Volume with Sampling**

```yaml
transforms:
  sample:
    type: sample
    inputs: [source]
    rate: 10  # Keep 1 in 10 logs
```

2. **Add Filtering**

```yaml
transforms:
  filter:
    type: filter
    inputs: [source]
    condition: |
      .level != "debug" &&
      !contains(string!(.message), "healthcheck")
```

3. **Tune Batch Sizes**

```yaml
sinks:
  elasticsearch:
    batch:
      max_bytes: 5242880  # 5MB (reduce from 10MB)
      timeout_secs: 30    # Flush more frequently
```

4. **Increase Resource Limits**

```yaml
sidecar:
  resources:
    requests:
      cpu: 200m
      memory: 256Mi
    limits:
      cpu: 1000m
      memory: 512Mi
```

---

### Issue 7: Injection Hash Mismatch

**Symptoms:**
- Deployments keep getting updated
- Unnecessary pod restarts
- Annotation shows different hash each reconciliation

**Diagnosis:**

```bash
# Check injection hash annotation
kubectl get deployment <name> -o jsonpath='{.metadata.annotations.vectorsidecar\.observability\.kontroloop\.ai/injected-hash}'

# Watch for changes
watch -n 2 'kubectl get deployment <name> -o jsonpath="{.metadata.annotations}"'
```

**Common Causes:**

1. **Non-Deterministic Configuration**

Using timestamp or random values in config.

**Solution:** Remove dynamic values from Vector configuration

2. **ConfigMap Version Changing**

**Solution:** This is expected when ConfigMap changes. To avoid, use inline config for static configurations.

---

## Debugging Steps

### Step-by-Step Debugging Process

1. **Verify Operator Health**

```bash
kubectl get deployment -n vector-sidecar-operator-system
kubectl logs -n vector-sidecar-operator-system deployment/vector-sidecar-operator-controller-manager --tail=100
```

2. **Check VectorSidecar Resource**

```bash
kubectl get vectorsidecar <name> -o yaml
kubectl describe vectorsidecar <name>
```

3. **Verify Label Matching**

```bash
# Get VectorSidecar selector
SELECTOR=$(kubectl get vectorsidecar <name> -o jsonpath='{.spec.selector.matchLabels}')

# Find matching deployments
kubectl get deployments -l <labels-from-selector>
```

4. **Check Deployment Annotations**

```bash
kubectl get deployment <name> -o jsonpath='{.metadata.annotations}' | jq .
```

5. **Inspect Pod Containers**

```bash
kubectl get pod <pod-name> -o jsonpath='{.spec.containers[*].name}'
kubectl describe pod <pod-name>
```

6. **Check Vector Container Logs**

```bash
kubectl logs <pod-name> -c vector --tail=100
kubectl logs <pod-name> -c vector --previous  # If crashed
```

---

## Log Analysis

### Operator Logs

**Successful Reconciliation:**
```
INFO    Reconciling VectorSidecar    {"name": "example", "namespace": "default"}
INFO    Found matching deployments    {"count": 3}
INFO    Injected sidecar successfully    {"deployment": "app-1"}
INFO    Updated VectorSidecar status    {"matched": 3, "injected": 3}
```

**Error Patterns:**

**ConfigMap Not Found:**
```
ERROR   Failed to get ConfigMap    {"name": "vector-config", "error": "not found"}
```

**RBAC Error:**
```
ERROR   Failed to update deployment    {"error": "forbidden: User cannot update resource"}
```

**Invalid Configuration:**
```
ERROR   Failed to build Vector container    {"error": "invalid config format"}
```

### Vector Logs

**Healthy Vector:**
```
INFO vector: Log level is enabled. level="info"
INFO vector: Loading configs. paths=["..."]
INFO vector: Vector has started. version="0.35.0"
INFO vector::topology: Running healthchecks.
INFO vector::topology: Healthchecks passed.
```

**Connection Issues:**
```
WARN vector::sinks::elasticsearch: Retrying request. error="Connection refused"
ERROR vector::sinks: Sink error. component_name="elasticsearch"
```

**Configuration Errors:**
```
ERROR vector: Configuration error. error="unknown field 'unknown_field'"
```

---

## Performance Issues

### Slow Log Processing

**Symptoms:**
- Increasing memory usage
- High CPU usage
- Logs delayed reaching destination

**Diagnosis:**

```bash
# Check internal metrics
kubectl port-forward <pod-name> 9598:9598
curl localhost:9598/metrics | grep -E "(component_received|component_sent|buffer_events)"
```

**Solutions:**

1. **Increase buffer size:**
```yaml
sinks:
  elasticsearch:
    buffer:
      type: disk
      max_size: 536870912  # 512MB
```

2. **Optimize transforms:**
```yaml
# Use efficient VRL functions
transforms:
  parse:
    type: remap
    source: |
      # Fast JSON parsing
      . = parse_json!(.message)
```

3. **Parallel processing:**
```yaml
env:
  - name: VECTOR_THREADS
    value: "4"  # Increase threads
```

---

## Getting Further Help

If you're still stuck:

1. **Check GitHub Issues:** [GitHub Issues](https://github.com/amitde789696/vector-sidecar-operator/issues)
2. **Search Discussions:** [GitHub Discussions](https://github.com/amitde789696/vector-sidecar-operator/discussions)
3. **Vector.dev Documentation:** [Vector Docs](https://vector.dev/docs/)

When asking for help, include:
- VectorSidecar YAML
- Operator logs
- Vector container logs
- `kubectl describe` output for affected resources

---

## Diagnostic Script

Save this script for quick diagnostics:

```bash
#!/bin/bash
# diagnose-vectorsidecar.sh

NAMESPACE=${1:-default}
VECTORSIDECAR_NAME=${2}

echo "=== Vector Sidecar Operator Diagnostics ==="
echo

echo "1. Operator Status:"
kubectl get deployment -n vector-sidecar-operator-system
echo

echo "2. CRD Status:"
kubectl get crd vectorsidecars.observability.kontroloop.ai
echo

if [ -n "$VECTORSIDECAR_NAME" ]; then
  echo "3. VectorSidecar Details:"
  kubectl get vectorsidecar $VECTORSIDECAR_NAME -n $NAMESPACE -o yaml
  echo

  echo "4. Matching Deployments:"
  LABELS=$(kubectl get vectorsidecar $VECTORSIDECAR_NAME -n $NAMESPACE -o jsonpath='{.spec.selector.matchLabels}' | jq -r 'to_entries | map("\(.key)=\(.value)") | join(",")')
  kubectl get deployments -n $NAMESPACE -l "$LABELS"
  echo
fi

echo "5. Operator Logs (last 20 lines):"
kubectl logs -n vector-sidecar-operator-system deployment/vector-sidecar-operator-controller-manager --tail=20
```

Usage:
```bash
chmod +x diagnose-vectorsidecar.sh
./diagnose-vectorsidecar.sh default my-vectorsidecar
```

---

## Related Documentation

- [Configuration Reference](configuration.md) - Complete field reference
- [Best Practices](best-practices.md) - Avoid common pitfalls
- [Architecture](architecture.md) - Understand how it works
- [Examples](../examples/) - Working configurations
