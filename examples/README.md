# Examples

This directory contains various examples demonstrating different use cases and configurations for the Vector Sidecar Injection Operator.

## Quick Start Examples

### 🚀 Basic Example

Start here if you're new to the operator.

**Files:** `vector-config.yaml`, `test-deployment.yaml`, `example-cr.yaml`

```bash
# Deploy all basic examples
kubectl apply -f vector-config.yaml
kubectl apply -f example-cr.yaml
kubectl apply -f test-deployment.yaml

# Verify the sidecar was injected
kubectl get vectorsidecar
kubectl get deployment test-app -o jsonpath='{.spec.template.spec.containers[*].name}'
```

**What it demonstrates:**
- Basic VectorSidecar configuration
- ConfigMap-based Vector configuration
- Simple label-based workload selection

---

## Advanced Examples

### 🏭 Production Example

Production-ready configuration with comprehensive settings.

**File:** `production-example.yaml`

```bash
kubectl create namespace production
kubectl apply -f production-example.yaml
```

**What it demonstrates:**
- Resource limits and requests
- Multiple volume mounts
- Environment variable configuration
- Complex Vector pipeline (parse, filter, multiple sinks)
- Elasticsearch and S3 sink configuration

**Key features:**
- ✅ Production-grade resource allocation
- ✅ Log aggregation to Elasticsearch
- ✅ Long-term storage in S3
- ✅ Log filtering and enrichment

---

### 🌍 Multi-Environment Example

Different configurations for dev, staging, and production.

**File:** `multi-environment.yaml`

```bash
kubectl create namespace development
kubectl create namespace staging
kubectl create namespace production

kubectl apply -f multi-environment.yaml
```

**What it demonstrates:**
- Environment-specific configurations
- Resource tuning per environment
- Inline vs ConfigMap-based configuration
- Different log verbosity levels

**Environment comparison:**

| Environment | CPU Request | Memory Request | Log Level | Output |
|-------------|-------------|----------------|-----------|--------|
| Development | 50m         | 64Mi           | debug     | Console |
| Staging     | 100m        | 128Mi          | info      | HTTP Backend |
| Production  | 200m        | 256Mi          | warn      | ConfigMap-based |

---

### 🎯 Selective Injection Example

Fine-grained workload targeting with complex label selectors.

**File:** `selective-injection.yaml`

```bash
kubectl apply -f selective-injection.yaml
```

**What it demonstrates:**
- Single label matching
- Multiple label matching (AND condition)
- `matchExpressions` for complex selection
- Opt-in vs opt-out patterns

**Selection strategies:**

1. **Single Label:** `tier: api`
2. **Multiple Labels (AND):** `criticality: high` AND `compliance: required`
3. **Match Expressions:** `app in (frontend, backend, api)` AND `monitoring exists`
4. **Opt-in Pattern:** Only inject when `vector-injection: enabled`

---

## Example Scenarios

### Scenario 1: Getting Started (5 minutes)

Perfect for first-time users.

```bash
# 1. Install the operator (assumes it's already deployed)
# 2. Deploy the basic example
kubectl apply -f vector-config.yaml
kubectl apply -f example-cr.yaml
kubectl apply -f test-deployment.yaml

# 3. Watch the magic happen
kubectl get vectorsidecar -w
```

### Scenario 2: Production Rollout (15 minutes)

Deploying to production with best practices.

```bash
# 1. Create production namespace
kubectl create namespace production

# 2. Review and customize the configuration
vim production-example.yaml
# Update: Elasticsearch endpoint, S3 bucket, resource limits

# 3. Deploy
kubectl apply -f production-example.yaml

# 4. Verify
kubectl -n production get vectorsidecar vector-production
kubectl -n production describe vectorsidecar vector-production
```

### Scenario 3: Multi-Environment Setup (20 minutes)

Set up consistent observability across all environments.

```bash
# 1. Create namespaces
kubectl create namespace development
kubectl create namespace staging
kubectl create namespace production

# 2. Deploy environment-specific configurations
kubectl apply -f multi-environment.yaml

# 3. Deploy test workloads in each namespace
for ns in development staging production; do
  kubectl -n $ns run test-app --image=nginx:latest --labels="environment=$ns,observability=enabled"
done

# 4. Verify injection in all environments
kubectl get vectorsidecar -A
```

---

## Testing Your Changes

After modifying examples or creating new ones:

```bash
# Validate YAML syntax
kubectl apply --dry-run=client -f <your-file>.yaml

# Apply and watch for errors
kubectl apply -f <your-file>.yaml
kubectl describe vectorsidecar <name>

# Check operator logs for any issues
kubectl logs -n vector-sidecar-operator-system deployment/vector-sidecar-operator-controller-manager
```

---

## Example Structure Reference

Each example file contains:

1. **VectorSidecar CR** - Defines injection rules and configuration
2. **ConfigMap** (optional) - External Vector configuration
3. **Deployment** (optional) - Sample workload to demonstrate injection

### Minimal VectorSidecar

```yaml
apiVersion: observability.kontroloop.ai/v1alpha1
kind: VectorSidecar
metadata:
  name: minimal-example
  namespace: default
spec:
  enabled: true
  selector:
    matchLabels:
      app: my-app
  sidecar:
    image: timberio/vector:0.35.0
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

---

## Common Patterns

### Pattern 1: Opt-In Injection

Only inject sidecars into deployments that explicitly request it.

```yaml
selector:
  matchLabels:
    vector-injection: enabled
```

### Pattern 2: Tier-Based Injection

Inject different configurations based on application tier.

```yaml
# API tier
selector:
  matchLabels:
    tier: api

# Frontend tier
selector:
  matchLabels:
    tier: frontend
```

### Pattern 3: Multi-Label Selection

Target specific combinations of labels.

```yaml
selector:
  matchLabels:
    environment: production
    team: platform
    criticality: high
```

### Pattern 4: Expression-Based Selection

Use complex matching logic.

```yaml
selector:
  matchExpressions:
    - key: app
      operator: In
      values: ["web", "api", "worker"]
    - key: monitoring
      operator: Exists
```

---

## Tips and Best Practices

### 💡 Configuration Tips

1. **Start small:** Begin with console output, then add remote sinks
2. **Use ConfigMaps:** Easier to update without changing CRs
3. **Test selectors:** Use `kubectl get deployments --show-labels` to verify
4. **Resource planning:** Start conservative, scale up based on metrics

### ⚠️ Common Pitfalls

1. **Selector conflicts:** Multiple VectorSidecar CRs matching the same deployment
2. **Missing ConfigMaps:** Ensure ConfigMaps exist before creating VectorSidecar CR
3. **Resource limits:** Too low can cause OOMKills, too high wastes resources
4. **Volume permissions:** Ensure Vector has read access to log paths

### 🔍 Debugging

```bash
# Check if selector matches any deployments
kubectl get deployments -l <your-selector>

# View VectorSidecar status
kubectl describe vectorsidecar <name>

# Check operator logs
kubectl logs -n vector-sidecar-operator-system deployment/vector-sidecar-operator-controller-manager -f

# View Vector logs in injected pod
kubectl logs <pod-name> -c vector
```

---

## Next Steps

- Read the [Getting Started Guide](../docs/getting-started.md)
- Review [Best Practices](../docs/best-practices.md)
- Check [Configuration Reference](../docs/configuration.md)
- See [Troubleshooting Guide](../docs/troubleshooting.md)

## Contributing Examples

Have a useful example? We'd love to include it! Please:

1. Follow the existing example structure
2. Add clear comments explaining the configuration
3. Include a scenario description in this README
4. Test thoroughly before submitting
5. Submit a PR with your example

See [CONTRIBUTING.md](../CONTRIBUTING.md) for more details.
