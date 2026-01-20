# Best Practices

This guide provides recommendations for deploying and operating the Vector Sidecar Operator in production environments.

## Table of Contents

- [Production Deployment](#production-deployment)
- [Configuration Management](#configuration-management)
- [Resource Planning](#resource-planning)
- [Security](#security)
- [Monitoring](#monitoring)
- [Troubleshooting](#troubleshooting)

## Production Deployment

### Operator Installation

**✅ DO:**
- Deploy operator in a dedicated namespace (e.g., `vector-sidecar-operator-system`)
- Run multiple replicas with leader election enabled
- Set appropriate resource limits
- Enable Pod Disruption Budgets (PDBs)
- Use specific image tags, not `latest`

**❌ DON'T:**
- Run in default namespace
- Use single replica in production
- Deploy without resource limits
- Use floating tags like `latest`

```yaml
# Recommended operator deployment
apiVersion: apps/v1
kind: Deployment
metadata:
  name: vector-sidecar-operator
  namespace: vector-sidecar-operator-system
spec:
  replicas: 2  # For high availability
  template:
    spec:
      containers:
      - name: manager
        image: ghcr.io/amitde789696/vector-sidecar-operator:v1.0.0  # Specific version
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
          limits:
            cpu: 500m
            memory: 512Mi
```

### Vector Image Selection

**Choose the right Vector image:**

| Image | Use Case | Size | Performance |
|-------|----------|------|-------------|
| `timberio/vector:0.35.0` | Development | ~500MB | Standard |
| `timberio/vector:0.35.0-alpine` | Production | ~200MB | Optimized |
| `timberio/vector:0.35.0-distroless` | Security-focused | ~180MB | Best security |

**Recommendation:** Use Alpine or Distroless for production

### Namespace Strategy

**Option 1: Separate Namespaces per Environment**
```
├── production/
│   ├── VectorSidecar (production config)
│   └── Deployments
├── staging/
│   ├── VectorSidecar (staging config)
│   └── Deployments
└── development/
    ├── VectorSidecar (dev config)
    └── Deployments
```

**Option 2: Shared Namespace with Labels**
```yaml
# All in same namespace, differentiate by labels
selector:
  matchLabels:
    environment: production
    tier: api
```

**Recommendation:** Use separate namespaces for better isolation

## Configuration Management

### ConfigMap vs Inline Configuration

**Use ConfigMap when:**
- ✅ Configuration is complex (>20 lines)
- ✅ Need to update config without changing CRs
- ✅ Sharing config across multiple VectorSidecars
- ✅ Managing config with GitOps (Flux, ArgoCD)

**Use Inline when:**
- ✅ Configuration is simple (<10 lines)
- ✅ Config is tightly coupled to CR
- ✅ Prototype/testing scenarios

```yaml
# ConfigMap approach (Recommended for production)
apiVersion: observability.amitde789696.io/v1alpha1
kind: VectorSidecar
metadata:
  name: production-vector
spec:
  sidecar:
    config:
      configMapRef:
        name: vector-production-config  # Managed separately
        key: vector.yaml

---
# Inline approach (Good for simple cases)
apiVersion: observability.amitde789696.io/v1alpha1
kind: VectorSidecar
metadata:
  name: dev-vector
spec:
  sidecar:
    config:
      inline: |
        sources:
          logs:
            type: kubernetes_logs
        sinks:
          stdout:
            type: console
            inputs: [logs]
```

### Vector Configuration Patterns

**Pattern 1: Layered Configuration**

Separate concerns by transform stages:

```yaml
sources:
  kubernetes_logs:
    type: kubernetes_logs

transforms:
  # Stage 1: Parse
  parse:
    type: remap
    inputs: [kubernetes_logs]
    source: |
      . = parse_json!(.message)

  # Stage 2: Enrich
  enrich:
    type: remap
    inputs: [parse]
    source: |
      .environment = "production"
      .cluster = get_env_var!("CLUSTER_NAME")

  # Stage 3: Filter
  filter:
    type: filter
    inputs: [enrich]
    condition: .level != "debug"

sinks:
  elasticsearch:
    type: elasticsearch
    inputs: [filter]
```

**Pattern 2: Multi-Sink Strategy**

Send logs to multiple destinations:

```yaml
sinks:
  # Primary: Real-time analysis
  elasticsearch:
    type: elasticsearch
    inputs: [transform]
    endpoint: "https://es.prod:9200"

  # Secondary: Long-term storage
  s3:
    type: aws_s3
    inputs: [transform]
    bucket: "logs-archive"

  # Tertiary: Alerting
  splunk:
    type: splunk_hec
    inputs: [filter_critical]
    endpoint: "https://splunk.prod:8088"
```

### Label Selector Best Practices

**✅ Good Selectors:**

```yaml
# Explicit and specific
selector:
  matchLabels:
    app: api-server
    environment: production
    vector-injection: enabled

# Using expressions for multiple apps
selector:
  matchExpressions:
    - key: tier
      operator: In
      values: [api, frontend]
    - key: observability
      operator: Exists
```

**❌ Bad Selectors:**

```yaml
# Too broad - matches everything
selector:
  matchLabels:
    app: "*"

# No specificity
selector: {}

# Single vague label
selector:
  matchLabels:
    enabled: "true"
```

**Recommendation:** Use multiple specific labels with `matchExpressions` for complex targeting

## Resource Planning

### Vector Resource Requirements

**Baseline recommendations by workload size:**

| Workload Size | CPU Request | Memory Request | CPU Limit | Memory Limit |
|---------------|-------------|----------------|-----------|--------------|
| Small (<10 pods) | 50m | 64Mi | 200m | 128Mi |
| Medium (10-50 pods) | 100m | 128Mi | 500m | 256Mi |
| Large (50-200 pods) | 200m | 256Mi | 1000m | 512Mi |
| XLarge (200+ pods) | 500m | 512Mi | 2000m | 1Gi |

**Example configuration:**

```yaml
sidecar:
  resources:
    requests:
      cpu: 100m       # Guaranteed allocation
      memory: 128Mi
    limits:
      cpu: 500m       # Burst capacity
      memory: 256Mi   # OOM kill threshold
```

### Resource Monitoring

Monitor these metrics to tune resources:

- **CPU usage:** Should stay below 80% of limit under normal load
- **Memory usage:** Should stay below 75% of limit
- **Container restarts:** Should be zero (indicates OOMKills if restarting)

```bash
# Check resource usage
kubectl top pod -l app=your-app --containers

# Check for OOMKills
kubectl get pod <pod-name> -o jsonpath='{.status.containerStatuses[?(@.name=="vector")].lastState.terminated.reason}'
```

### Scaling Considerations

**Horizontal scaling:**
- Vector sidecar scales with your application pods automatically
- No additional configuration needed

**Vertical scaling:**
- Adjust resources based on log volume
- Monitor and tune using actual metrics

```yaml
# Update resources in VectorSidecar
kubectl patch vectorsidecar production-vector -p '
{
  "spec": {
    "sidecar": {
      "resources": {
        "requests": {"cpu": "200m", "memory": "256Mi"},
        "limits": {"cpu": "1000m", "memory": "512Mi"}
      }
    }
  }
}'
```

## Security

### RBAC Configuration

**Principle of Least Privilege:**

```yaml
# Operator ServiceAccount - minimal permissions
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: vector-sidecar-operator
  namespace: production
rules:
- apiGroups: ["apps"]
  resources: ["deployments"]
  verbs: ["get", "list", "watch", "update", "patch"]
- apiGroups: ["observability.amitde789696.io"]
  resources: ["vectorsidecars"]
  verbs: ["get", "list", "watch", "update", "patch"]
- apiGroups: ["observability.amitde789696.io"]
  resources: ["vectorsidecars/status"]
  verbs: ["update", "patch"]
```

### Pod Security

**Vector container security:**

```yaml
sidecar:
  securityContext:
    runAsNonRoot: true
    runAsUser: 65532
    allowPrivilegeEscalation: false
    capabilities:
      drop:
        - ALL
    readOnlyRootFilesystem: true  # When possible
```

### Secrets Management

**For sensitive configuration (API keys, credentials):**

```yaml
# Use Secrets, not ConfigMaps
env:
  - name: ELASTICSEARCH_PASSWORD
    valueFrom:
      secretKeyRef:
        name: vector-secrets
        key: es-password

# Or use external secrets operator
env:
  - name: API_KEY
    valueFrom:
      secretKeyRef:
        name: external-secret
        key: api-key
```

### Network Policies

**Restrict Vector's network access:**

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: vector-sidecar-policy
spec:
  podSelector:
    matchLabels:
      observability: vector
  policyTypes:
    - Egress
  egress:
    # Allow DNS
    - to:
        - namespaceSelector:
            matchLabels:
              name: kube-system
      ports:
        - protocol: UDP
          port: 53
    # Allow Elasticsearch
    - to:
        - podSelector:
            matchLabels:
              app: elasticsearch
      ports:
        - protocol: TCP
          port: 9200
```

## Monitoring

### Operator Health Checks

**Monitor operator metrics:**

```yaml
# Prometheus ServiceMonitor
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: vector-sidecar-operator
spec:
  selector:
    matchLabels:
      app: vector-sidecar-operator
  endpoints:
    - port: metrics
      interval: 30s
```

**Key metrics to watch:**
- `controller_runtime_reconcile_total` - Total reconciliations
- `controller_runtime_reconcile_errors_total` - Failed reconciliations
- `workqueue_depth` - Pending reconciliation queue depth

### VectorSidecar Status Monitoring

**Check VectorSidecar health:**

```bash
# List all VectorSidecars with status
kubectl get vectorsidecar -A -o wide

# Check specific VectorSidecar
kubectl describe vectorsidecar production-vector

# Alert on unhealthy state
kubectl get vectorsidecar production-vector -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}'
# Should return "True"
```

### Vector Sidecar Metrics

**Monitor Vector container health:**

```bash
# Check Vector logs for errors
kubectl logs <pod-name> -c vector --tail=100

# Vector exposes metrics on port 9598
kubectl port-forward <pod-name> 9598:9598
curl localhost:9598/metrics
```

## Troubleshooting

### Common Issues and Solutions

#### Issue 1: Sidecar Not Injected

**Symptoms:**
- `MATCHED: 0` in VectorSidecar status
- Deployment has no Vector container

**Solution:**
```bash
# Check deployment labels
kubectl get deployment <name> --show-labels

# Verify selector matches
kubectl get vectorsidecar <name> -o jsonpath='{.spec.selector}'

# Fix: Add matching labels to deployment
kubectl label deployment <name> observability=vector
```

#### Issue 2: ConfigMap Not Found

**Symptoms:**
- `ConfigValid: False` in status
- Error message about missing ConfigMap

**Solution:**
```bash
# Check if ConfigMap exists
kubectl get configmap <name> -n <namespace>

# Create if missing
kubectl create configmap vector-config --from-file=vector.yaml
```

#### Issue 3: High Resource Usage

**Symptoms:**
- Vector container using too much CPU/memory
- OOMKills (container restarts)

**Solution:**
```bash
# Check actual usage
kubectl top pod <pod-name> --containers

# Increase limits
kubectl patch vectorsidecar <name> -p '{"spec":{"sidecar":{"resources":{"limits":{"memory":"512Mi"}}}}}'

# Check for log volume
# May need to add sampling/filtering
```

#### Issue 4: Logs Not Appearing in Sink

**Symptoms:**
- Vector running but logs not reaching destination
- No errors in Vector logs

**Solution:**
```bash
# Check Vector logs
kubectl logs <pod-name> -c vector | grep -i error

# Verify Vector configuration
kubectl exec <pod-name> -c vector -- cat /etc/vector/vector.yaml

# Test connectivity
kubectl exec <pod-name> -c vector -- curl <sink-endpoint>

# Enable debug logging
env:
  - name: VECTOR_LOG
    value: debug
```

### Debug Checklist

When something isn't working:

1. ✅ Operator running? `kubectl get pod -n vector-sidecar-operator-system`
2. ✅ CRDs installed? `kubectl get crd vectorsidecars.observability.amitde789696.io`
3. ✅ VectorSidecar valid? `kubectl get vectorsidecar -o wide`
4. ✅ Labels match? `kubectl get deployment --show-labels`
5. ✅ ConfigMap exists? `kubectl get configmap`
6. ✅ Resources sufficient? `kubectl top pod --containers`
7. ✅ Network connectivity? Test from Vector container
8. ✅ Logs clean? Check both operator and Vector logs

## Performance Tips

1. **Use compression:** Enable gzip compression for remote sinks
2. **Batch smartly:** Configure appropriate batch sizes
3. **Filter early:** Remove unnecessary logs before enrichment
4. **Use sampling:** For high-volume logs, use sampling transforms
5. **Alpine images:** Use Alpine-based images for smaller footprint

```yaml
# Example optimized sink configuration
sinks:
  elasticsearch:
    type: elasticsearch
    inputs: [filtered_logs]
    compression: gzip          # Reduce network bandwidth
    batch:
      max_bytes: 10485760      # 10MB batches
      timeout_secs: 60         # Flush every 60s
    request:
      retry_attempts: 5
      retry_initial_backoff_secs: 1
```

## Backup and Disaster Recovery

### Backup VectorSidecar CRs

```bash
# Export all VectorSidecar resources
kubectl get vectorsidecar -A -o yaml > vectorsidecars-backup.yaml

# Restore from backup
kubectl apply -f vectorsidecars-backup.yaml
```

### ConfigMap Versioning

Keep ConfigMaps in Git for version control:

```bash
# Commit changes
git add vector-configs/
git commit -m "Update Vector config for production"
git push

# Deploy using GitOps (Flux/ArgoCD)
```

## Conclusion

Following these best practices will help you run a stable, secure, and efficient Vector Sidecar Operator deployment in production. Remember to:

- 📊 Monitor continuously
- 🔒 Apply security principles
- 📈 Plan resources appropriately
- 🧪 Test configuration changes
- 📝 Document your setup

For more information, see:
- [Architecture Guide](architecture.md)
- [Troubleshooting Guide](troubleshooting.md)
- [Configuration Reference](configuration.md)
