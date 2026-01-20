# Getting Started with Vector Sidecar Operator

This guide will walk you through installing and using the Vector Sidecar Injection Operator for the first time.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [First Deployment](#first-deployment)
- [Verification](#verification)
- [Next Steps](#next-steps)

## Prerequisites

Before you begin, ensure you have:

- **Kubernetes cluster** (v1.20 or later)
  - Minikube, kind, or any production cluster
  - At least 2GB RAM available
  - kubectl configured and working

- **Cluster permissions**
  - Ability to create Custom Resource Definitions (CRDs)
  - Namespace admin permissions (minimum)
  - Cluster admin for production installations

- **Tools installed**
  - kubectl (v1.20+)
  - make (for building from source)
  - Go 1.21+ (if developing)

### Verify Prerequisites

```bash
# Check kubectl connection
kubectl cluster-info

# Check your permissions
kubectl auth can-i create customresourcedefinitions --all-namespaces

# Check available resources
kubectl top nodes
```

## Installation

### Option 1: Quick Install (Recommended)

Use the pre-built manifests for the fastest installation:

```bash
# Install CRDs and operator
kubectl apply -f https://github.com/amitde789696/vector-sidecar-operator/releases/latest/download/install.yaml

# Verify installation
kubectl get deployment -n vector-sidecar-operator-system
```

### Option 2: Install from Source

Clone and build from source for development or customization:

```bash
# Clone the repository
git clone https://github.com/amitde789696/vector-sidecar-operator.git
cd vector-sidecar-operator

# Install CRDs
make install

# Deploy the operator
make deploy IMG=ghcr.io/amitde789696/vector-sidecar-operator:latest
```

### Option 3: Local Development

Run the operator locally (ideal for development):

```bash
# Install CRDs only
make install

# Run operator on your machine
make run
```

### Verify Installation

Check that the operator is running:

```bash
# Check operator deployment
kubectl get deployment -n vector-sidecar-operator-system

# Check operator logs
kubectl logs -n vector-sidecar-operator-system deployment/vector-sidecar-operator-controller-manager

# Verify CRDs are installed
kubectl get crd vectorsidecars.observability.amitde789696.io
```

Expected output:
```
NAME                                              CREATED AT
vectorsidecars.observability.amitde789696.io     2026-01-20T10:00:00Z
```

## First Deployment

Let's deploy your first Vector sidecar! We'll create:
1. A simple application
2. A Vector configuration
3. A VectorSidecar resource to inject Vector

### Step 1: Create a Test Application

Create a simple nginx deployment:

```bash
kubectl create deployment test-app \
  --image=nginx:latest \
  --replicas=1

# Add the observability label
kubectl label deployment test-app observability=vector

# Verify the deployment
kubectl get deployment test-app --show-labels
```

### Step 2: Create Vector Configuration

Create a ConfigMap with Vector configuration:

```bash
kubectl apply -f - <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: vector-config
  namespace: default
data:
  vector.yaml: |
    # Simple Vector configuration
    sources:
      kubernetes_logs:
        type: kubernetes_logs

    transforms:
      parse_logs:
        type: remap
        inputs:
          - kubernetes_logs
        source: |
          # Add timestamp and environment
          .timestamp = now()
          .environment = "development"

    sinks:
      stdout:
        type: console
        inputs:
          - parse_logs
        encoding:
          codec: json
EOF
```

### Step 3: Create VectorSidecar Resource

Now create the VectorSidecar resource to inject Vector into your app:

```bash
kubectl apply -f - <<EOF
apiVersion: observability.amitde789696.io/v1alpha1
kind: VectorSidecar
metadata:
  name: my-first-vector
  namespace: default
spec:
  enabled: true

  # Target deployments with observability=vector label
  selector:
    matchLabels:
      observability: vector

  # Vector sidecar configuration
  sidecar:
    name: vector
    image: timberio/vector:0.35.0
    config:
      configMapRef:
        name: vector-config
        key: vector.yaml
    resources:
      requests:
        cpu: 100m
        memory: 128Mi
      limits:
        cpu: 500m
        memory: 256Mi
EOF
```

## Verification

### Check VectorSidecar Status

```bash
# Get VectorSidecar resources
kubectl get vectorsidecar

# Expected output:
# NAME              ENABLED   MATCHED   INJECTED   READY   AGE
# my-first-vector   true      1         1          True    30s

# Detailed information
kubectl describe vectorsidecar my-first-vector
```

The status should show:
- **ENABLED**: true
- **MATCHED**: 1 (found one matching deployment)
- **INJECTED**: 1 (successfully injected)
- **READY**: True

### Verify Sidecar Injection

Check that the Vector sidecar was added to your deployment:

```bash
# List containers in the deployment
kubectl get deployment test-app -o jsonpath='{.spec.template.spec.containers[*].name}'

# Expected output: nginx vector
```

You should see two containers: `nginx` (your app) and `vector` (the sidecar).

### Check Vector Logs

View logs from the Vector sidecar:

```bash
# Get the pod name
POD=$(kubectl get pod -l observability=vector -o jsonpath='{.items[0].metadata.name}')

# View Vector container logs
kubectl logs $POD -c vector

# You should see Vector initialization and log processing
```

### Test Log Collection

Generate some logs and watch Vector process them:

```bash
# Generate logs from your app
kubectl exec -it $POD -c nginx -- sh -c 'for i in 1 2 3; do echo "Test log $i"; done'

# Watch Vector process the logs
kubectl logs $POD -c vector --tail=20
```

## Next Steps

Congratulations! You've successfully:
- ✅ Installed the Vector Sidecar Operator
- ✅ Created your first VectorSidecar resource
- ✅ Injected Vector into a deployment
- ✅ Verified log collection

### Learn More

Now that you have the basics working, explore:

1. **[Configuration Reference](configuration.md)** - Learn about all available options
2. **[Best Practices](best-practices.md)** - Production deployment recommendations
3. **[Examples](../examples/)** - More complex use cases
4. **[Architecture](architecture.md)** - Understand how the operator works

### Common Next Steps

#### Add Remote Sink

Update your Vector configuration to send logs to a remote system:

```yaml
sinks:
  elasticsearch:
    type: elasticsearch
    inputs: [parse_logs]
    endpoint: "https://elasticsearch:9200"
```

#### Target Multiple Applications

Update the selector to match more deployments:

```yaml
selector:
  matchExpressions:
    - key: team
      operator: In
      values: ["platform", "api"]
```

#### Add Resource Monitoring

Configure Vector to collect metrics:

```yaml
sources:
  host_metrics:
    type: host_metrics
```

## Troubleshooting

### Sidecar Not Injected

**Problem:** Status shows MATCHED: 0

**Solution:** Check deployment labels
```bash
kubectl get deployment test-app --show-labels
# Ensure it has: observability=vector
```

### ConfigMap Not Found Error

**Problem:** Status shows ConfigValid: False

**Solution:** Verify ConfigMap exists
```bash
kubectl get configmap vector-config
# Create it if missing
```

### Operator Not Running

**Problem:** No VectorSidecar status updates

**Solution:** Check operator logs
```bash
kubectl logs -n vector-sidecar-operator-system deployment/vector-sidecar-operator-controller-manager
```

### Permission Denied

**Problem:** CRD installation fails

**Solution:** Ensure cluster-admin permissions
```bash
kubectl auth can-i create crd --all-namespaces
```

## Clean Up

To remove the test resources:

```bash
# Delete VectorSidecar (this will remove injected sidecars)
kubectl delete vectorsidecar my-first-vector

# Delete test application
kubectl delete deployment test-app

# Delete ConfigMap
kubectl delete configmap vector-config

# Uninstall operator (optional)
make undeploy  # If installed from source
# OR
kubectl delete -f https://github.com/amitde789696/vector-sidecar-operator/releases/latest/download/install.yaml
```

## Getting Help

- **Issues:** [GitHub Issues](https://github.com/amitde789696/vector-sidecar-operator/issues)
- **Discussions:** [GitHub Discussions](https://github.com/amitde789696/vector-sidecar-operator/discussions)
- **Documentation:** [docs/](.)

## What's Next?

- 📖 Read the [Architecture Guide](architecture.md) to understand the internals
- 🏭 Review [Best Practices](best-practices.md) for production deployments
- 🔧 Check out [Advanced Examples](../examples/) for complex scenarios
- 🤝 Learn how to [Contribute](../CONTRIBUTING.md)
