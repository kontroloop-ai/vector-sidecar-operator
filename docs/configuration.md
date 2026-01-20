# Configuration Reference

Complete API reference for the VectorSidecar Custom Resource Definition.

## Table of Contents

- [VectorSidecar API](#vectorsidecar-api)
- [Field Reference](#field-reference)
- [Configuration Examples](#configuration-examples)
- [Advanced Configurations](#advanced-configurations)

## VectorSidecar API

The VectorSidecar resource defines how and where Vector sidecars should be injected.

### API Group and Version

```yaml
apiVersion: observability.kontroloop.ai/v1alpha1
kind: VectorSidecar
```

### Basic Structure

```yaml
apiVersion: observability.kontroloop.ai/v1alpha1
kind: VectorSidecar
metadata:
  name: string
  namespace: string
  labels: map[string]string
  annotations: map[string]string
spec:
  enabled: boolean
  selector: LabelSelector
  sidecar: SidecarSpec
  volumes: []Volume
status:
  matchedDeployments: int32
  injectedDeployments: int32
  conditions: []Condition
  lastReconcileTime: Time
```

## Field Reference

### Spec Fields

#### `enabled` (required)

**Type:** `boolean`

**Description:** Controls whether sidecar injection is active for this VectorSidecar resource.

**Values:**
- `true`: Inject Vector sidecar into matching deployments
- `false`: Remove Vector sidecar from previously injected deployments

**Example:**
```yaml
spec:
  enabled: true
```

**Notes:**
- Setting to `false` will trigger removal of injected sidecars
- Useful for temporary disabling without deleting the CR

---

#### `selector` (required)

**Type:** `LabelSelector`

**Description:** Kubernetes label selector to target specific Deployments for injection.

**Fields:**
- `matchLabels`: Map of key-value pairs (AND logic)
- `matchExpressions`: List of label selector requirements

**Example - Simple:**
```yaml
spec:
  selector:
    matchLabels:
      app: my-app
      environment: production
```

**Example - Complex:**
```yaml
spec:
  selector:
    matchExpressions:
      - key: tier
        operator: In
        values: [api, frontend]
      - key: monitoring
        operator: Exists
```

**Supported Operators:**
- `In`: Label value must be in the values list
- `NotIn`: Label value must not be in the values list
- `Exists`: Label key must exist (any value)
- `DoesNotExist`: Label key must not exist

---

#### `sidecar` (required)

**Type:** `SidecarSpec`

**Description:** Configuration for the Vector sidecar container.

**Fields:**

##### `sidecar.name`

**Type:** `string`

**Default:** `"vector"`

**Description:** Name of the sidecar container.

```yaml
sidecar:
  name: vector
```

---

##### `sidecar.image`

**Type:** `string`

**Required:** Yes

**Description:** Docker image for Vector.

**Recommended Images:**
- `timberio/vector:0.35.0` - Standard image
- `timberio/vector:0.35.0-alpine` - Alpine-based (smaller)
- `timberio/vector:0.35.0-distroless` - Distroless (most secure)

```yaml
sidecar:
  image: timberio/vector:0.35.0-alpine
```

---

##### `sidecar.imagePullPolicy`

**Type:** `string`

**Default:** `"IfNotPresent"`

**Values:** `Always`, `IfNotPresent`, `Never`

```yaml
sidecar:
  imagePullPolicy: IfNotPresent
```

---

##### `sidecar.config`

**Type:** `VectorConfigSource`

**Description:** Source of Vector configuration.

**Options:**

**Option 1: ConfigMap Reference**
```yaml
sidecar:
  config:
    configMapRef:
      name: vector-config
      key: vector.yaml
```

**Option 2: Inline Configuration**
```yaml
sidecar:
  config:
    inline: |
      sources:
        kubernetes_logs:
          type: kubernetes_logs
      sinks:
        stdout:
          type: console
          inputs: [kubernetes_logs]
```

---

##### `sidecar.env`

**Type:** `[]EnvVar`

**Description:** Environment variables for the Vector container.

```yaml
sidecar:
  env:
    - name: VECTOR_LOG
      value: info
    - name: VECTOR_THREADS
      value: "2"
    - name: DATABASE_PASSWORD
      valueFrom:
        secretKeyRef:
          name: db-secret
          key: password
```

---

##### `sidecar.resources`

**Type:** `ResourceRequirements`

**Description:** CPU and memory resource configuration.

```yaml
sidecar:
  resources:
    requests:
      cpu: 100m
      memory: 128Mi
    limits:
      cpu: 500m
      memory: 256Mi
```

**Guidelines:**
- **Requests:** Guaranteed allocation
- **Limits:** Maximum allowed usage
- See [Best Practices](best-practices.md#resource-planning) for sizing recommendations

---

##### `sidecar.volumeMounts`

**Type:** `[]VolumeMount`

**Description:** Volume mounts for the Vector container.

```yaml
sidecar:
  volumeMounts:
    - name: logs
      mountPath: /var/log
      readOnly: true
    - name: vector-data
      mountPath: /var/lib/vector
```

---

##### `sidecar.securityContext`

**Type:** `SecurityContext`

**Description:** Security settings for the Vector container.

```yaml
sidecar:
  securityContext:
    runAsNonRoot: true
    runAsUser: 65532
    allowPrivilegeEscalation: false
    capabilities:
      drop:
        - ALL
    readOnlyRootFilesystem: true
```

---

#### `volumes`

**Type:** `[]Volume`

**Description:** Additional volumes to add to the pod.

```yaml
volumes:
  - name: logs
    emptyDir: {}
  - name: config
    configMap:
      name: app-config
  - name: data
    persistentVolumeClaim:
      claimName: vector-data
```

---

### Status Fields

The operator automatically populates these fields.

#### `status.matchedDeployments`

**Type:** `int32`

**Description:** Number of Deployments matching the selector.

#### `status.injectedDeployments`

**Type:** `int32`

**Description:** Number of Deployments successfully injected.

#### `status.conditions`

**Type:** `[]Condition`

**Description:** Status conditions.

**Condition Types:**
- `Ready`: Overall operational status
- `ConfigValid`: Configuration validation passed
- `Error`: Error occurred during reconciliation

#### `status.lastReconcileTime`

**Type:** `metav1.Time`

**Description:** Timestamp of last reconciliation.

---

## Configuration Examples

### Minimal Configuration

```yaml
apiVersion: observability.kontroloop.ai/v1alpha1
kind: VectorSidecar
metadata:
  name: minimal
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

### Production Configuration

```yaml
apiVersion: observability.kontroloop.ai/v1alpha1
kind: VectorSidecar
metadata:
  name: production-vector
  namespace: production
  labels:
    environment: production
    team: platform
spec:
  enabled: true

  selector:
    matchLabels:
      environment: production
      observability: enabled

  sidecar:
    name: vector
    image: timberio/vector:0.35.0-alpine
    imagePullPolicy: IfNotPresent

    config:
      configMapRef:
        name: vector-production-config
        key: vector.yaml

    env:
      - name: VECTOR_LOG
        value: warn
      - name: VECTOR_THREADS
        value: "4"
      - name: ENVIRONMENT
        value: production
      - name: ELASTICSEARCH_PASSWORD
        valueFrom:
          secretKeyRef:
            name: elasticsearch-creds
            key: password

    resources:
      requests:
        cpu: 200m
        memory: 256Mi
      limits:
        cpu: 1000m
        memory: 512Mi

    volumeMounts:
      - name: varlog
        mountPath: /var/log
        readOnly: true
      - name: vector-data
        mountPath: /var/lib/vector

    securityContext:
      runAsNonRoot: true
      runAsUser: 65532
      allowPrivilegeEscalation: false
      capabilities:
        drop:
          - ALL

  volumes:
    - name: varlog
      hostPath:
        path: /var/log
        type: Directory
    - name: vector-data
      emptyDir:
        sizeLimit: 1Gi
```

## Advanced Configurations

### Multi-Sink Configuration

Send logs to multiple destinations:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: vector-multi-sink
data:
  vector.yaml: |
    sources:
      kubernetes_logs:
        type: kubernetes_logs

    transforms:
      parse_and_enrich:
        type: remap
        inputs: [kubernetes_logs]
        source: |
          . = parse_json!(.message)
          .environment = "production"
          .cluster = get_env_var!("CLUSTER_NAME")

    sinks:
      # Primary: Real-time analysis
      elasticsearch:
        type: elasticsearch
        inputs: [parse_and_enrich]
        endpoint: "https://elasticsearch:9200"
        mode: bulk
        compression: gzip

      # Secondary: Long-term storage
      s3:
        type: aws_s3
        inputs: [parse_and_enrich]
        bucket: "logs-archive"
        compression: gzip
        encoding:
          codec: json

      # Tertiary: Metrics
      prometheus:
        type: prometheus_exporter
        inputs: [parse_and_enrich]
        address: "0.0.0.0:9598"
```

### Conditional Injection Based on Namespace

```yaml
# Use namespace labels for selection
apiVersion: observability.kontroloop.ai/v1alpha1
kind: VectorSidecar
metadata:
  name: namespace-wide
  namespace: production
spec:
  enabled: true
  selector:
    matchLabels:
      inject-vector: "true"  # Opt-in per deployment
  sidecar:
    image: timberio/vector:0.35.0
    config:
      configMapRef:
        name: default-vector-config
        key: vector.yaml
```

### High-Performance Configuration

For high-volume log collection:

```yaml
apiVersion: observability.kontroloop.ai/v1alpha1
kind: VectorSidecar
metadata:
  name: high-performance
spec:
  enabled: true
  selector:
    matchLabels:
      workload-type: high-volume
  sidecar:
    image: timberio/vector:0.35.0-alpine
    env:
      - name: VECTOR_THREADS
        value: "8"
      - name: VECTOR_REQUIRE_HEALTHY
        value: "true"
    resources:
      requests:
        cpu: 500m
        memory: 512Mi
      limits:
        cpu: 2000m
        memory: 1Gi
    config:
      inline: |
        sources:
          logs:
            type: kubernetes_logs

        transforms:
          # Sampling to reduce volume
          sample:
            type: sample
            inputs: [logs]
            rate: 10  # 1 in 10 logs

        sinks:
          elasticsearch:
            type: elasticsearch
            inputs: [sample]
            endpoint: "https://elasticsearch:9200"
            batch:
              max_bytes: 10485760  # 10MB
              timeout_secs: 30
            compression: gzip
```

### Security-Focused Configuration

Minimal permissions and isolation:

```yaml
apiVersion: observability.kontroloop.ai/v1alpha1
kind: VectorSidecar
metadata:
  name: secure-vector
spec:
  enabled: true
  selector:
    matchLabels:
      security-level: high
  sidecar:
    image: timberio/vector:0.35.0-distroless
    config:
      configMapRef:
        name: secure-vector-config
        key: vector.yaml
    securityContext:
      runAsNonRoot: true
      runAsUser: 65532
      runAsGroup: 65532
      allowPrivilegeEscalation: false
      readOnlyRootFilesystem: true
      capabilities:
        drop:
          - ALL
      seccompProfile:
        type: RuntimeDefault
    resources:
      requests:
        cpu: 100m
        memory: 128Mi
      limits:
        cpu: 500m
        memory: 256Mi
```

## Validation Rules

The operator validates configurations:

1. **Required Fields:**
   - `spec.enabled` must be set
   - `spec.selector` must have at least one label matcher
   - `spec.sidecar.image` is required

2. **ConfigMap Validation:**
   - Referenced ConfigMaps must exist
   - ConfigMap key must exist

3. **Resource Validation:**
   - Limits must be >= requests
   - Valid resource quantities

4. **Selector Validation:**
   - At least one matchLabel or matchExpression required

## Best Practices

1. **Use ConfigMaps for production:** Easier to update without changing CRs
2. **Set resource limits:** Prevent resource exhaustion
3. **Use specific image tags:** Avoid surprises with `latest`
4. **Enable security context:** Run as non-root user
5. **Monitor status conditions:** Watch for errors

## Troubleshooting

### Check Configuration Validity

```bash
# Get VectorSidecar status
kubectl get vectorsidecar <name> -o yaml

# Check conditions
kubectl get vectorsidecar <name> -o jsonpath='{.status.conditions}'
```

### Common Issues

**Issue: ConfigMap not found**
```
Condition: ConfigValid = False
Message: ConfigMap "vector-config" not found
```

**Solution:** Create the ConfigMap before the VectorSidecar

**Issue: No deployments matched**
```
Status: matchedDeployments = 0
```

**Solution:** Verify deployment labels match the selector

## Next Steps

- [Getting Started Guide](getting-started.md) - First-time setup
- [Best Practices](best-practices.md) - Production recommendations
- [Examples](../examples/) - More configuration examples
- [Architecture](architecture.md) - How it works internally
