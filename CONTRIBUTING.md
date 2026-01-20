# Contributing to Vector Sidecar Operator

Thank you for your interest in contributing to the Vector Sidecar Operator! This document provides guidelines and instructions for contributing.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Workflow](#development-workflow)
- [Pull Request Process](#pull-request-process)
- [Coding Standards](#coding-standards)
- [Testing](#testing)
- [Documentation](#documentation)
- [Community](#community)

## Code of Conduct

This project follows the [CNCF Code of Conduct](https://github.com/cncf/foundation/blob/master/code-of-conduct.md). By participating, you are expected to uphold this code. Please report unacceptable behavior to the project maintainers.

## Getting Started

### Prerequisites

Before you begin, ensure you have:

- **Go 1.21+** installed ([download](https://golang.org/dl/))
- **Docker** for building images ([install](https://docs.docker.com/get-docker/))
- **kubectl** configured with a test cluster ([install](https://kubernetes.io/docs/tasks/tools/))
- **make** for running build targets
- **Git** for version control

### Fork and Clone

1. Fork the repository on GitHub
2. Clone your fork locally:

```bash
git clone https://github.com/YOUR_USERNAME/vector-sidecar-operator.git
cd vector-sidecar-operator
```

3. Add the upstream repository:

```bash
git remote add upstream https://github.com/amitde789696/vector-sidecar-operator.git
```

4. Verify remotes:

```bash
git remote -v
# origin    https://github.com/YOUR_USERNAME/vector-sidecar-operator.git (fetch)
# origin    https://github.com/YOUR_USERNAME/vector-sidecar-operator.git (push)
# upstream  https://github.com/amitde789696/vector-sidecar-operator.git (fetch)
# upstream  https://github.com/amitde789696/vector-sidecar-operator.git (push)
```

### Set Up Development Environment

```bash
# Install dependencies
go mod download

# Install CRDs into your cluster
make install

# Run tests to verify setup
make test

# Run operator locally
make run
```

## Development Workflow

### 1. Create a Branch

Always create a feature branch for your work:

```bash
# Sync with upstream
git fetch upstream
git checkout main
git merge upstream/main

# Create feature branch
git checkout -b feature/your-feature-name

# Or for bug fixes
git checkout -b fix/issue-description
```

**Branch naming conventions:**
- `feature/` - New features
- `fix/` - Bug fixes
- `docs/` - Documentation changes
- `refactor/` - Code refactoring
- `test/` - Test improvements

### 2. Make Your Changes

Follow these guidelines:

- **Keep changes focused:** One feature or fix per PR
- **Write tests:** Add unit tests for new functionality
- **Update docs:** Keep documentation in sync with code
- **Follow Go conventions:** Use `gofmt`, `go vet`, and `golint`

### 3. Test Your Changes

Run the full test suite:

```bash
# Run all tests
make test

# Run tests with coverage
make test
go tool cover -html=cover.out

# Run linters
go vet ./...
gofmt -s -w .

# Test locally with a real cluster
make install  # Install CRDs
make run      # Run operator locally
```

### 4. Commit Your Changes

Write clear, descriptive commit messages:

```bash
git add .
git commit -m "Add feature: brief description

Detailed explanation of what changed and why.
Fixes #123"
```

**Commit message format:**
```
<type>: <subject>

<body>

<footer>
```

**Types:**
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `test`: Test additions/changes
- `refactor`: Code refactoring
- `chore`: Maintenance tasks

**Example:**
```
feat: add support for init containers in VectorSidecar

This adds a new field `initContainers` to the VectorSidecar spec,
allowing users to inject init containers alongside the Vector sidecar.

This is useful for setup tasks like fetching configuration from
external sources or running migration scripts.

Fixes #45
```

### 5. Push and Create PR

```bash
# Push to your fork
git push origin feature/your-feature-name

# Create PR on GitHub
# Go to https://github.com/amitde789696/vector-sidecar-operator
# Click "New Pull Request"
```

## Pull Request Process

### PR Template

When creating a PR, include:

```markdown
## Description
Brief description of changes

## Type of Change
- [ ] Bug fix
- [ ] New feature
- [ ] Breaking change
- [ ] Documentation update

## Testing
- [ ] Added unit tests
- [ ] Tested locally
- [ ] Updated documentation

## Checklist
- [ ] Code follows project style
- [ ] Tests pass locally
- [ ] Documentation updated
- [ ] No new warnings
```

### PR Review Process

1. **Automated Checks:** CI/CD must pass
   - Tests
   - Linting
   - Build verification

2. **Code Review:** At least one maintainer approval
   - Code quality
   - Test coverage
   - Documentation

3. **Updates:** Address review comments
   ```bash
   # Make changes
   git add .
   git commit -m "Address review comments"
   git push origin feature/your-feature-name
   ```

4. **Merge:** Maintainer will merge once approved

### What to Expect

- **Response time:** Within 48 hours for initial review
- **Review iterations:** 1-3 rounds typically
- **Merge time:** 1-2 weeks for most PRs

## Coding Standards

### Go Style Guide

Follow the [Effective Go](https://golang.org/doc/effective_go) guidelines and:

- **Formatting:** Use `gofmt` (automatically done by editors)
- **Naming:** Use descriptive names, camelCase for functions
- **Comments:** Document exported functions and types
- **Error handling:** Always check errors, provide context

**Example:**

```go
// InjectSidecar injects a Vector sidecar container into the given deployment.
// It returns an error if the injection fails or if the deployment is invalid.
func (r *VectorSidecarReconciler) InjectSidecar(
    ctx context.Context,
    deployment *appsv1.Deployment,
    vectorSidecar *observabilityv1alpha1.VectorSidecar,
) error {
    if deployment == nil {
        return fmt.Errorf("deployment cannot be nil")
    }

    // Calculate injection hash
    hash, err := calculateInjectionHash(vectorSidecar)
    if err != nil {
        return fmt.Errorf("failed to calculate hash: %w", err)
    }

    // ... rest of implementation
}
```

### Code Organization

```
vector-sidecar-operator/
├── api/v1alpha1/          # CRD type definitions
│   └── vectorsidecar_types.go
├── controllers/           # Controller logic
│   ├── vectorsidecar_controller.go
│   └── vectorsidecar_controller_test.go
├── config/                # Kubernetes manifests
│   ├── crd/              # Generated CRDs
│   ├── rbac/             # RBAC policies
│   └── samples/          # Sample CRs
├── docs/                  # Documentation
├── examples/              # Usage examples
└── main.go               # Entry point
```

### API Compatibility

Follow Kubernetes API guidelines:

- **Never remove fields:** Deprecate instead
- **Add fields as optional:** With sensible defaults
- **Version CRDs properly:** Use v1alpha1, v1beta1, v1
- **Document changes:** In API docs and CHANGELOG

**Example - Adding a field:**

```go
// VectorSidecarSpec defines the desired state of VectorSidecar
type VectorSidecarSpec struct {
    // Existing fields...

    // NewField is an optional field that does X.
    // +optional
    // +kubebuilder:default=default-value
    NewField string `json:"newField,omitempty"`
}
```

## Testing

### Unit Tests

Write tests for all new code:

```go
func TestInjectSidecar(t *testing.T) {
    tests := []struct {
        name    string
        input   *appsv1.Deployment
        want    bool
        wantErr bool
    }{
        {
            name: "successful injection",
            input: &appsv1.Deployment{
                // ... test deployment
            },
            want:    true,
            wantErr: false,
        },
        // More test cases...
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := InjectSidecar(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("InjectSidecar() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if got != tt.want {
                t.Errorf("InjectSidecar() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

### Integration Tests

Test controller behavior:

```go
var _ = Describe("VectorSidecar Controller", func() {
    Context("When creating a VectorSidecar", func() {
        It("Should inject sidecar into matching deployments", func() {
            By("Creating a test deployment")
            deployment := &appsv1.Deployment{
                // ... deployment spec
            }
            Expect(k8sClient.Create(ctx, deployment)).To(Succeed())

            By("Creating a VectorSidecar")
            vectorSidecar := &observabilityv1alpha1.VectorSidecar{
                // ... vectorsidecar spec
            }
            Expect(k8sClient.Create(ctx, vectorSidecar)).To(Succeed())

            By("Checking that sidecar was injected")
            Eventually(func() int {
                var dep appsv1.Deployment
                k8sClient.Get(ctx, types.NamespacedName{
                    Name:      deployment.Name,
                    Namespace: deployment.Namespace,
                }, &dep)
                return len(dep.Spec.Template.Spec.Containers)
            }, timeout, interval).Should(Equal(2))
        })
    })
})
```

### Test Coverage

Maintain test coverage above 70%:

```bash
# Check coverage
make test
go tool cover -func=cover.out

# View coverage HTML report
go tool cover -html=cover.out
```

## Documentation

### Code Documentation

- **Document exports:** All exported functions, types, and constants
- **Explain why:** Not just what the code does
- **Examples:** Provide usage examples for complex functions

### User Documentation

Update relevant docs when:

- **Adding features:** Update README, getting-started.md
- **Changing APIs:** Update configuration.md
- **Fixing bugs:** Update troubleshooting.md
- **Architecture changes:** Update architecture.md

### Documentation Structure

```
docs/
├── getting-started.md     # First-time user guide
├── architecture.md        # Design and internals
├── configuration.md       # API reference
├── best-practices.md      # Production recommendations
├── troubleshooting.md     # Common issues
└── development.md         # Development guide
```

## Community

### Getting Help

- **GitHub Issues:** [Report bugs or request features](https://github.com/amitde789696/vector-sidecar-operator/issues)
- **Discussions:** [Ask questions and share ideas](https://github.com/amitde789696/vector-sidecar-operator/discussions)
- **Pull Requests:** [Contribute code](https://github.com/amitde789696/vector-sidecar-operator/pulls)

### Issue Labels

We use labels to organize issues:

- `good first issue` - Great for new contributors
- `help wanted` - Community help needed
- `bug` - Something isn't working
- `enhancement` - New feature request
- `documentation` - Documentation improvements
- `question` - Further information requested

### Finding Issues to Work On

Look for issues labeled:
- **good first issue** - Perfect for first-time contributors
- **help wanted** - Maintainers need help with these

Comment on an issue to claim it:
```
I'd like to work on this! I'll submit a PR by [date].
```

## Release Process

(For maintainers)

1. Update version in relevant files
2. Update CHANGELOG.md
3. Create and push tag
4. GitHub Actions builds and publishes release
5. Update documentation for new version

## Recognition

Contributors will be:
- Listed in CONTRIBUTORS.md
- Mentioned in release notes
- Invited to join maintainers (for significant contributions)

## Questions?

If you have questions about contributing:

1. Check existing [Issues](https://github.com/amitde789696/vector-sidecar-operator/issues) and [Discussions](https://github.com/amitde789696/vector-sidecar-operator/discussions)
2. Open a new [Discussion](https://github.com/amitde789696/vector-sidecar-operator/discussions/new)
3. Reach out to maintainers

## Thank You!

Your contributions make this project better for everyone. We appreciate your time and effort! 🎉

---

## Quick Reference

### Common Commands

```bash
# Development
make test                  # Run tests
make build                 # Build binary
make run                   # Run locally
make install               # Install CRDs
make manifests             # Generate manifests

# Code quality
gofmt -s -w .             # Format code
go vet ./...              # Static analysis
go mod tidy               # Clean dependencies

# Git workflow
git fetch upstream         # Get latest from upstream
git rebase upstream/main   # Rebase on main
git push origin branch     # Push to your fork
```

### Useful Links

- [Kubebuilder Book](https://book.kubebuilder.io/)
- [controller-runtime docs](https://pkg.go.dev/sigs.k8s.io/controller-runtime)
- [Kubernetes API Conventions](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md)
- [Vector.dev Documentation](https://vector.dev/docs/)
