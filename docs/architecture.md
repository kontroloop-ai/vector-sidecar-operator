# Architecture Guide

This document provides a deep dive into the Vector Sidecar Operator's architecture, design decisions, and implementation patterns.

## Table of Contents

- [Overview](#overview)
- [Components](#components)
- [Reconciliation Loop](#reconciliation-loop)
- [Injection Mechanism](#injection-mechanism)
- [State Management](#state-management)
- [Design Decisions](#design-decisions)

## Overview

The Vector Sidecar Operator is a Kubernetes operator that automatically manages Vector sidecar container injection into existing Deployments based on declarative configuration.

### Architecture Diagram

```
┌─────────────────────────────────────────────────────────────┐
│                     Kubernetes Cluster                      │
│                                                             │
│  ┌────────────────────────────────────────────────────┐   │
│  │         Vector Sidecar Operator                    │   │
│  │                                                     │   │
│  │  ┌──────────────────────────────────────────────┐ │   │
│  │  │     Controller Manager                        │ │   │
│  │  │                                               │ │   │
│  │  │  1. Watch VectorSidecar CRs                  │ │   │
│  │  │  2. List matching Deployments                │ │   │
│  │  │  3. Calculate injection hash                 │ │   │
│  │  │  4. Inject/Update sidecars                   │ │   │
│  │  │  5. Update CR status                         │ │   │
│  │  └──────────────────────────────────────────────┘ │   │
│  └────────────────────────────────────────────────────┘   │
│          │                         ▲                        │
│          │ watches                 │ updates                │
│          ▼                         │                        │
│  ┌──────────────────┐      ┌──────────────────┐           │
│  │  VectorSidecar   │      │   Deployments    │           │
│  │  Custom Resource │      │  (with sidecars) │           │
│  └──────────────────┘      └──────────────────┘           │
│          │                         │                        │
│          │ references              │ uses                   │
│          ▼                         ▼                        │
│  ┌──────────────────┐      ┌──────────────────┐           │
│  │   ConfigMaps     │      │      Pods        │           │
│  │ (Vector config)  │      │ [app | vector]   │           │
│  └──────────────────┘      └──────────────────┘           │
└─────────────────────────────────────────────────────────────┘
```

## Components

### 1. Custom Resource Definition (CRD)

**VectorSidecar CRD** defines the desired state for sidecar injection:

```yaml
apiVersion: observability.kontroloop.ai/v1alpha1
kind: VectorSidecar
metadata:
  name: example
spec:
  enabled: bool              # Enable/disable injection
  selector: LabelSelector    # Target deployments
  sidecar: SidecarConfig     # Vector container spec
  volumes: []Volume          # Additional volumes
```

**Location:** `api/v1alpha1/vectorsidecar_types.go`

**Key fields:**
- `Spec`: User's desired state
- `Status`: Operator-maintained current state
  - `MatchedDeployments`: Count of matching deployments
  - `InjectedDeployments`: Count of successfully injected
  - `Conditions`: Status conditions (Ready, ConfigValid, Error)

### 2. Controller

**VectorSidecarReconciler** implements the reconciliation logic.

**Location:** `controllers/vectorsidecar_controller.go`

**Responsibilities:**
- Watch VectorSidecar resources for changes
- Discover and match Deployments based on label selectors
- Calculate injection configuration hash
- Inject or update Vector sidecar containers
- Manage finalizers for cleanup
- Update status conditions

**Controller Configuration:**
```go
func (r *VectorSidecarReconciler) SetupWithManager(mgr ctrl.Manager) error {
    return ctrl.NewControllerManagedBy(mgr).
        For(&observabilityv1alpha1.VectorSidecar{}).
        Owns(&appsv1.Deployment{}).  // Watch owned deployments
        Complete(r)
}
```

### 3. Reconciliation Manager

The controller-runtime manager provides:
- Client for accessing Kubernetes API
- Caching layer for efficient reads
- Event-driven reconciliation triggers
- Leader election (in multi-replica deployments)

## Reconciliation Loop

The operator uses a level-triggered reconciliation loop pattern.

### Reconciliation Flow

```
┌─────────────────────────────────────────────────────────┐
│                  Reconcile Request                      │
│            (triggered by watch event)                   │
└───────────────────┬─────────────────────────────────────┘
                    │
                    ▼
    ┌───────────────────────────────┐
    │  1. Fetch VectorSidecar CR    │
    │     - Check if deleted        │◄──┐
    │     - Handle finalizers       │   │
    └───────────┬───────────────────┘   │
                │                        │
                ▼                        │
    ┌───────────────────────────────┐   │
    │  2. Validate Configuration    │   │
    │     - Check ConfigMap exists  │   │
    │     - Validate selector       │   │ Requeue
    └───────────┬───────────────────┘   │ on error
                │                        │
                ▼                        │
    ┌───────────────────────────────┐   │
    │  3. Find Matching Deployments │   │
    │     - List by label selector  │   │
    │     - Filter by namespace     │   │
    └───────────┬───────────────────┘   │
                │                        │
                ▼                        │
    ┌───────────────────────────────┐   │
    │  4. Process Each Deployment   │   │
    │     - Calculate config hash   │   │
    │     - Check if update needed  │   │
    │     - Inject/remove sidecar   │───┘
    └───────────┬───────────────────┘
                │
                ▼
    ┌───────────────────────────────┐
    │  5. Update CR Status          │
    │     - Set matched count       │
    │     - Set injected count      │
    │     - Update conditions       │
    └───────────┬───────────────────┘
                │
                ▼
            ┌───────┐
            │ Done  │
            └───────┘
```

### Trigger Conditions

Reconciliation is triggered when:

1. **VectorSidecar CR changes**
   - Created, updated, or deleted
   - Spec changes (image, config, selector)

2. **Deployment changes** (owned by VectorSidecar)
   - Label changes
   - Deployment created/deleted

3. **ConfigMap changes** (referenced by VectorSidecar)
   - Configuration updates

4. **Periodic resync**
   - Default: every 10 hours
   - Ensures eventual consistency

## Injection Mechanism

### Hash-Based Change Detection

The operator uses SHA256 hashing to detect configuration changes:

```go
func calculateInjectionHash(vectorSidecar *VectorSidecar) string {
    data := struct {
        Image         string
        Config        VectorConfig
        VolumeMounts  []VolumeMount
        Resources     ResourceRequirements
        Env           []EnvVar
    }{
        // ... populate from VectorSidecar spec
    }

    hash := sha256.Sum256([]byte(fmt.Sprintf("%+v", data)))
    return hex.EncodeToString(hash[:])
}
```

**Hash is stored in:** `vectorsidecar.observability.kontroloop.ai/injected-hash` annotation

**Benefits:**
- ✅ Prevents unnecessary pod restarts
- ✅ Idempotent operations
- ✅ Drift detection
- ✅ Efficient reconciliation

### Injection Process

```go
func (r *VectorSidecarReconciler) injectSidecar(
    deployment *appsv1.Deployment,
    vectorSidecar *VectorSidecar,
) error {
    // 1. Calculate new hash
    newHash := calculateInjectionHash(vectorSidecar)

    // 2. Check current hash
    currentHash := deployment.Annotations[InjectedHashAnnotation]
    if currentHash == newHash {
        return nil  // Already up to date
    }

    // 3. Build sidecar container
    sidecarContainer := buildVectorContainer(vectorSidecar)

    // 4. Inject into deployment
    deployment.Spec.Template.Spec.Containers = append(
        deployment.Spec.Template.Spec.Containers,
        sidecarContainer,
    )

    // 5. Add volumes if specified
    deployment.Spec.Template.Spec.Volumes = append(
        deployment.Spec.Template.Spec.Volumes,
        vectorSidecar.Spec.Volumes...
    )

    // 6. Update annotations
    deployment.Annotations[InjectedAnnotation] = "true"
    deployment.Annotations[InjectedHashAnnotation] = newHash
    deployment.Annotations[SidecarNameAnnotation] = vectorSidecar.Name

    // 7. Apply update
    return r.Update(ctx, deployment)
}
```

### Removal Process

When `enabled: false` or VectorSidecar is deleted:

```go
func (r *VectorSidecarReconciler) removeSidecar(
    deployment *appsv1.Deployment,
    sidecarName string,
) error {
    // 1. Filter out Vector container
    containers := []corev1.Container{}
    for _, c := range deployment.Spec.Template.Spec.Containers {
        if c.Name != sidecarName {
            containers = append(containers, c)
        }
    }
    deployment.Spec.Template.Spec.Containers = containers

    // 2. Remove annotations
    delete(deployment.Annotations, InjectedAnnotation)
    delete(deployment.Annotations, InjectedHashAnnotation)
    delete(deployment.Annotations, SidecarNameAnnotation)

    // 3. Apply update
    return r.Update(ctx, deployment)
}
```

## State Management

### Annotations

The operator uses annotations to track injection state:

| Annotation | Purpose | Example Value |
|------------|---------|---------------|
| `vectorsidecar.../injected` | Marks deployment as injected | `"true"` |
| `vectorsidecar.../injected-hash` | Configuration hash | `"a1b2c3d4..."` |
| `vectorsidecar.../sidecar-name` | Managing VectorSidecar name | `"vector-prod"` |
| `vectorsidecar.../configmap-version` | ConfigMap resourceVersion | `"12345"` |

### Finalizers

Finalizers ensure proper cleanup before deletion:

```yaml
metadata:
  finalizers:
    - vectorsidecar.observability.kontroloop.ai/finalizer
```

**Cleanup sequence:**
1. User deletes VectorSidecar CR
2. Kubernetes sets `deletionTimestamp`
3. Operator reconciles deletion
4. Operator removes sidecars from all deployments
5. Operator removes finalizer
6. Kubernetes deletes the CR

### Status Conditions

Standard Kubernetes conditions pattern:

```go
type VectorSidecarStatus struct {
    Conditions []metav1.Condition `json:"conditions,omitempty"`

    MatchedDeployments  int32 `json:"matchedDeployments"`
    InjectedDeployments int32 `json:"injectedDeployments"`
    LastReconcileTime   *metav1.Time `json:"lastReconcileTime,omitempty"`
}
```

**Condition types:**
- **Ready**: Overall health status
- **ConfigValid**: Configuration validation passed
- **Error**: Error occurred during reconciliation

## Design Decisions

### 1. Why Deployments Only?

**Decision:** Only support Deployment injection (not StatefulSets, DaemonSets, etc.)

**Rationale:**
- ✅ Deployments are the most common workload type
- ✅ Simpler implementation and testing
- ✅ Reduces scope and complexity
- 🔄 Future: Can extend to other types if needed

### 2. Why Hash-Based Updates?

**Decision:** Use SHA256 hash to detect configuration changes

**Rationale:**
- ✅ Prevents unnecessary pod restarts
- ✅ Idempotent reconciliation
- ✅ Efficient comparison (O(1) instead of deep equality)
- ✅ Detects drift automatically

**Alternative considered:** Deep equality check
- ❌ Slower performance
- ❌ More complex comparison logic

### 3. Why Label Selectors?

**Decision:** Use Kubernetes label selectors for targeting

**Rationale:**
- ✅ Native Kubernetes pattern
- ✅ Flexible and powerful
- ✅ Familiar to users
- ✅ Supports complex expressions

**Alternative considered:** Namespace-wide injection
- ❌ Less granular control
- ❌ Harder to opt-out

### 4. Why Namespace-Scoped?

**Decision:** VectorSidecar operates within namespace boundaries

**Rationale:**
- ✅ Better security isolation
- ✅ Follows principle of least privilege
- ✅ Easier RBAC management
- ✅ Reduces blast radius

**Alternative considered:** Cluster-scoped
- ❌ Security concerns
- ❌ More complex RBAC

### 5. Why Finalizers?

**Decision:** Use finalizers for cleanup

**Rationale:**
- ✅ Ensures proper cleanup
- ✅ Prevents orphaned sidecars
- ✅ Standard Kubernetes pattern
- ✅ Handles async deletion properly

### 6. Why ConfigMap for Vector Config?

**Decision:** Support both ConfigMap and inline configuration

**Rationale:**
- ✅ ConfigMap: easier updates, separate lifecycle
- ✅ Inline: simpler for small configs
- ✅ Flexibility for different use cases

## Performance Considerations

### Caching

Controller-runtime provides automatic caching:

- **List operations:** Served from cache
- **Get operations:** Served from cache
- **Write operations:** Go directly to API server

### Watch Optimization

Only watch relevant resources:
```go
For(&VectorSidecar{}).
Owns(&Deployment{}).
Complete(r)
```

### Reconciliation Efficiency

- ✅ Early returns when no change needed
- ✅ Batch status updates
- ✅ Hash-based comparison avoids deep inspection

### Resource Limits

Operator resource recommendations:
```yaml
resources:
  requests:
    cpu: 100m
    memory: 128Mi
  limits:
    cpu: 500m
    memory: 512Mi
```

## Testing Strategy

### Unit Tests

Test individual functions in isolation:
- Hash calculation
- Container building
- Selector matching
- Status updates

### Integration Tests

Use envtest for controller testing:
```go
testEnv = &envtest.Environment{
    CRDDirectoryPaths: []string{filepath.Join("..", "config", "crd", "bases")},
}
```

### End-to-End Tests

Manual testing scenarios:
1. Basic injection
2. Configuration updates
3. Deployment scaling
4. Cleanup on deletion
5. Multiple VectorSidecar CRs

## Security Considerations

### RBAC

Minimal required permissions:
```yaml
rules:
- apiGroups: ["apps"]
  resources: ["deployments"]
  verbs: ["get", "list", "watch", "update", "patch"]
- apiGroups: ["observability.kontroloop.ai"]
  resources: ["vectorsidecars"]
  verbs: ["get", "list", "watch", "update", "patch"]
```

### Pod Security

- Vector runs as non-root (UID 65532)
- No privileged escalation
- Read-only root filesystem (where possible)

## Future Enhancements

Potential improvements:

1. **Multi-workload support**
   - StatefulSets, DaemonSets
   - Jobs, CronJobs

2. **Advanced health checks**
   - Vector health probes
   - Automatic restart on failure

3. **Metrics integration**
   - Prometheus metrics
   - Grafana dashboards

4. **Webhook validation**
   - Validate VectorSidecar on create
   - Prevent invalid configurations

5. **Multi-cluster support**
   - Cross-cluster injection
   - Centralized configuration

## References

- [Kubernetes Operator Pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/)
- [controller-runtime](https://github.com/kubernetes-sigs/controller-runtime)
- [Vector.dev Documentation](https://vector.dev/docs/)
- [Kubebuilder Book](https://book.kubebuilder.io/)
