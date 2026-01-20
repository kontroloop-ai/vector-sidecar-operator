# Development Guide

Guide for developers who want to contribute to or extend the Vector Sidecar Operator.

## Table of Contents

- [Development Setup](#development-setup)
- [Project Structure](#project-structure)
- [Building and Running](#building-and-running)
- [Testing](#testing)
- [Making Changes](#making-changes)
- [Debugging](#debugging)

## Development Setup

### Prerequisites

Install the required tools:

- **Go 1.21+** - [Download](https://golang.org/dl/)
- **Docker** - [Install](https://docs.docker.com/get-docker/)
- **kubectl** - [Install](https://kubernetes.io/docs/tasks/tools/)
- **kind** or **minikube** - Local Kubernetes cluster
- **make** - Build automation
- **kustomize** - Kubernetes manifest management
- **controller-gen** - CRD generation (installed automatically)

### Fork and Clone

```bash
# Fork on GitHub, then clone your fork
git clone https://github.com/YOUR_USERNAME/vector-sidecar-operator.git
cd vector-sidecar-operator

# Add upstream remote
git remote add upstream https://github.com/amitde789696/vector-sidecar-operator.git

# Verify remotes
git remote -v
```

### Install Dependencies

```bash
# Download Go dependencies
go mod download

# Verify everything compiles
go build -o bin/manager main.go
```

### Set Up Local Cluster

**Option 1: kind**

```bash
kind create cluster --name vector-sidecar-dev

# Verify
kubectl cluster-info --context kind-vector-sidecar-dev
```

**Option 2: minikube**

```bash
minikube start --profile vector-sidecar-dev

# Verify
kubectl cluster-info
```

## Project Structure

```
vector-sidecar-operator/
├── api/
│   └── v1alpha1/
│       ├── groupversion_info.go      # API group and version
│       ├── vectorsidecar_types.go    # CRD type definitions
│       └── zz_generated.deepcopy.go  # Generated code
├── controllers/
│   ├── vectorsidecar_controller.go   # Main reconciliation logic
│   └── vectorsidecar_controller_test.go  # Controller tests
├── config/
│   ├── crd/                          # CRD manifests
│   ├── rbac/                         # RBAC policies
│   ├── manager/                      # Operator deployment
│   ├── default/                      # Kustomize default
│   └── samples/                      # Sample CRs
├── hack/
│   ├── boilerplate.go.txt           # License header
│   ├── fix-crd-claims.sh            # CRD validation fix
│   └── fix-crd-claims.py            # Python CRD fixer
├── docs/                             # Documentation
├── examples/                         # Usage examples
├── main.go                           # Entry point
├── Makefile                          # Build targets
├── Dockerfile                        # Container image
└── go.mod                            # Go dependencies
```

### Key Files

**`api/v1alpha1/vectorsidecar_types.go`**
- Defines VectorSidecar CRD structure
- Spec and Status definitions
- Kubebuilder markers for CRD generation

**`controllers/vectorsidecar_controller.go`**
- Reconciliation loop implementation
- Deployment discovery and injection logic
- Status management
- Finalizer handling

**`main.go`**
- Operator initialization
- Manager setup
- Controller registration

**`Makefile`**
- Build and development targets
- Code generation commands
- Testing and deployment

## Building and Running

### Generate Code and Manifests

After changing API types:

```bash
# Generate DeepCopy methods
make generate

# Generate CRD manifests and RBAC
make manifests
```

### Build the Binary

```bash
# Build operator binary
make build

# Binary will be in bin/manager
./bin/manager --help
```

### Run Locally (Outside Cluster)

Best for rapid development:

```bash
# Install CRDs into cluster
make install

# Run operator locally (connects to cluster via kubeconfig)
make run

# In another terminal, create test resources
kubectl apply -f config/samples/
```

**Benefits:**
- Fast iteration (no image build/push)
- Easy debugging (local debugger)
- See logs directly in terminal

### Run in Cluster

Deploy operator as a deployment:

```bash
# Build and push image
make docker-build docker-push IMG=<your-registry>/vector-sidecar-operator:dev

# Deploy to cluster
make deploy IMG=<your-registry>/vector-sidecar-operator:dev

# Check operator status
kubectl get deployment -n vector-sidecar-operator-system

# View logs
kubectl logs -n vector-sidecar-operator-system deployment/vector-sidecar-operator-controller-manager -f
```

### Undeploy

```bash
# Remove operator from cluster
make undeploy

# Remove CRDs
make uninstall
```

## Testing

### Run Unit Tests

```bash
# Run all tests
make test

# Run with verbose output
go test -v ./...

# Run specific test
go test -v ./controllers -run TestVectorSidecarReconcile

# Generate coverage report
make test
go tool cover -html=cover.out
```

### Run Integration Tests

Integration tests use envtest (simulated API server):

```bash
# Set up envtest
make envtest

# Run integration tests
KUBEBUILDER_ASSETS="$(pwd)/bin/k8s/1.26.0-linux-amd64" go test ./controllers -v
```

### Write Tests

**Unit Test Example:**

```go
// controllers/vectorsidecar_controller_test.go
func TestCalculateInjectionHash(t *testing.T) {
    tests := []struct {
        name     string
        vs       *observabilityv1alpha1.VectorSidecar
        expected string
    }{
        {
            name: "basic configuration",
            vs: &observabilityv1alpha1.VectorSidecar{
                Spec: observabilityv1alpha1.VectorSidecarSpec{
                    Sidecar: observabilityv1alpha1.SidecarSpec{
                        Image: "timberio/vector:0.35.0",
                    },
                },
            },
            expected: "a1b2c3d4...", // Known hash
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            hash := calculateInjectionHash(tt.vs)
            if hash != tt.expected {
                t.Errorf("got %s, want %s", hash, tt.expected)
            }
        })
    }
}
```

**Integration Test Example:**

```go
var _ = Describe("VectorSidecar Controller", func() {
    Context("When creating a VectorSidecar", func() {
        It("Should inject sidecar into matching deployments", func() {
            ctx := context.Background()

            // Create deployment
            deployment := &appsv1.Deployment{
                ObjectMeta: metav1.ObjectMeta{
                    Name:      "test-deployment",
                    Namespace: "default",
                    Labels: map[string]string{
                        "app": "test",
                    },
                },
                Spec: appsv1.DeploymentSpec{
                    Selector: &metav1.LabelSelector{
                        MatchLabels: map[string]string{"app": "test"},
                    },
                    Template: corev1.PodTemplateSpec{
                        ObjectMeta: metav1.ObjectMeta{
                            Labels: map[string]string{"app": "test"},
                        },
                        Spec: corev1.PodSpec{
                            Containers: []corev1.Container{
                                {Name: "app", Image: "nginx"},
                            },
                        },
                    },
                },
            }
            Expect(k8sClient.Create(ctx, deployment)).To(Succeed())

            // Create VectorSidecar
            vs := &observabilityv1alpha1.VectorSidecar{
                ObjectMeta: metav1.ObjectMeta{
                    Name:      "test-vector",
                    Namespace: "default",
                },
                Spec: observabilityv1alpha1.VectorSidecarSpec{
                    Enabled: true,
                    Selector: metav1.LabelSelector{
                        MatchLabels: map[string]string{"app": "test"},
                    },
                    Sidecar: observabilityv1alpha1.SidecarSpec{
                        Image: "timberio/vector:0.35.0",
                    },
                },
            }
            Expect(k8sClient.Create(ctx, vs)).To(Succeed())

            // Verify injection
            Eventually(func() int {
                var dep appsv1.Deployment
                k8sClient.Get(ctx, client.ObjectKeyFromObject(deployment), &dep)
                return len(dep.Spec.Template.Spec.Containers)
            }, timeout, interval).Should(Equal(2)) // app + vector
        })
    })
})
```

## Making Changes

### Workflow

1. **Create Feature Branch**

```bash
git checkout -b feature/my-feature
```

2. **Make Changes**

Edit code, add tests, update docs.

3. **Generate Code**

```bash
make generate manifests
```

4. **Run Tests**

```bash
make test
go vet ./...
```

5. **Test Locally**

```bash
make install
make run
# Test with real resources
```

6. **Commit**

```bash
git add .
git commit -m "feat: add feature description"
```

7. **Push and Create PR**

```bash
git push origin feature/my-feature
```

### Adding a New Field

Example: Adding `imagePullSecrets` to VectorSidecar

1. **Update API Types**

```go
// api/v1alpha1/vectorsidecar_types.go
type SidecarSpec struct {
    // ... existing fields ...

    // ImagePullSecrets for pulling Vector image
    // +optional
    ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`
}
```

2. **Regenerate**

```bash
make generate manifests
```

3. **Update Controller**

```go
// controllers/vectorsidecar_controller.go
func (r *VectorSidecarReconciler) injectSidecar(...) {
    // Add imagePullSecrets to pod spec
    if len(vectorSidecar.Spec.Sidecar.ImagePullSecrets) > 0 {
        deployment.Spec.Template.Spec.ImagePullSecrets = append(
            deployment.Spec.Template.Spec.ImagePullSecrets,
            vectorSidecar.Spec.Sidecar.ImagePullSecrets...,
        )
    }
}
```

4. **Add Tests**

```go
func TestImagePullSecrets(t *testing.T) {
    // Test the new field
}
```

5. **Update Documentation**

- Add to `docs/configuration.md`
- Add example to `examples/`
- Update `README.md` if notable

## Debugging

### Debug Locally with Delve

```bash
# Install delve
go install github.com/go-delve/delve/cmd/dlv@latest

# Run with debugger
dlv debug ./main.go -- \
  --metrics-bind-address=:8080 \
  --health-probe-bind-address=:8081

# Set breakpoints in another terminal
(dlv) break controllers/vectorsidecar_controller.go:123
(dlv) continue
```

### Debug with VS Code

Create `.vscode/launch.json`:

```json
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "Debug Operator",
      "type": "go",
      "request": "launch",
      "mode": "debug",
      "program": "${workspaceFolder}/main.go",
      "args": [
        "--metrics-bind-address=:8080",
        "--health-probe-bind-address=:8081"
      ],
      "env": {},
      "showLog": true
    }
  ]
}
```

Set breakpoints in editor, press F5 to start debugging.

### Enable Verbose Logging

```bash
# Run with verbose logs
make run ARGS="--zap-log-level=debug"

# Or set env var
VERBOSE=1 make run
```

### Debug in Cluster

```bash
# Port-forward for debugging tools
kubectl port-forward -n vector-sidecar-operator-system \
  deployment/vector-sidecar-operator-controller-manager 8080:8080

# Access pprof
curl http://localhost:8080/debug/pprof/

# CPU profile
curl http://localhost:8080/debug/pprof/profile?seconds=30 > cpu.prof
go tool pprof cpu.prof
```

### Common Issues

**Issue: "no matches for kind VectorSidecar"**

Solution: Install CRDs first
```bash
make install
```

**Issue: "Failed to watch resources"**

Solution: Check RBAC permissions
```bash
kubectl describe clusterrole vector-sidecar-operator-manager-role
```

**Issue: "Context deadline exceeded"**

Solution: Increase reconciliation timeout in tests

## Code Style

### Follow Go Best Practices

- Use `gofmt` for formatting
- Run `go vet` for static analysis
- Follow [Effective Go](https://golang.org/doc/effective_go)

```bash
# Format code
gofmt -w .

# Vet code
go vet ./...

# Run linters (if golangci-lint installed)
golangci-lint run
```

### Kubebuilder Markers

Use appropriate markers in code:

```go
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Enabled",type=boolean,JSONPath=`.spec.enabled`
// +kubebuilder:resource:shortName=vs
type VectorSidecar struct {
    // ...
}

// +kubebuilder:validation:Enum=Always;IfNotPresent;Never
// +kubebuilder:default="IfNotPresent"
ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`
```

## Release Process

(For maintainers)

```bash
# Update version
export VERSION=v1.0.0

# Tag release
git tag $VERSION
git push origin $VERSION

# Build and push images
make docker-build docker-push IMG=ghcr.io/amitde789696/vector-sidecar-operator:$VERSION

# Generate release artifacts
make build-installer IMG=ghcr.io/amitde789696/vector-sidecar-operator:$VERSION

# Create GitHub release with dist/install.yaml
gh release create $VERSION dist/install.yaml \
  --title "Release $VERSION" \
  --notes "Release notes here"
```

## Useful Make Targets

```bash
make help              # Show all targets
make test              # Run tests
make build             # Build binary
make run               # Run locally
make docker-build      # Build image
make deploy            # Deploy to cluster
make install           # Install CRDs
make uninstall         # Remove CRDs
make manifests         # Generate CRDs
make generate          # Generate code
make fmt               # Format code
make vet               # Run go vet
```

## Resources

- [Kubebuilder Book](https://book.kubebuilder.io/)
- [controller-runtime](https://pkg.go.dev/sigs.k8s.io/controller-runtime)
- [Kubernetes API Conventions](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md)
- [Operator SDK](https://sdk.operatorframework.io/)
- [Vector.dev Docs](https://vector.dev/docs/)

## Getting Help

- [GitHub Issues](https://github.com/amitde789696/vector-sidecar-operator/issues)
- [Discussions](https://github.com/amitde789696/vector-sidecar-operator/discussions)
- [Contributing Guide](../CONTRIBUTING.md)
