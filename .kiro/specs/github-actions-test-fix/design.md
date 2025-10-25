# Design Document

## Overview

The GitHub Actions CI pipeline is failing with exit code 1 despite tests passing locally. Analysis reveals several issues:

1. **Invalid Go Version**: The CI configuration specifies Go 1.24, which doesn't exist (latest stable is Go 1.21)
2. **Coverage Threshold Logic**: The coverage check uses shell arithmetic that may behave differently across environments
3. **Dependency Conflicts**: The `bc` command used in coverage calculations may not be available in GitHub Actions runners
4. **Race Condition Potential**: Multiple jobs running coverage checks simultaneously could interfere with each other

## Architecture

### Current State Analysis

The current CI pipeline has four jobs:
- `lint`: Runs golangci-lint (working correctly)
- `test`: Runs tests with coverage and uploads to Codecov
- `build`: Builds binaries for multiple platforms
- `coverage`: Separate coverage threshold check (problematic)

### Proposed Solution Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Lint Job      │    │   Test Job      │    │   Build Job     │
│                 │    │                 │    │                 │
│ - golangci-lint │    │ - Run tests     │    │ - Build binary  │
│ - Go 1.21       │    │ - Coverage      │    │ - Multi-platform│
│                 │    │ - Threshold     │    │ - CGO disabled  │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

## Components and Interfaces

### 1. Go Version Management
- **Component**: GitHub Actions Go setup
- **Interface**: `actions/setup-go@v4`
- **Configuration**: Use Go 1.21.x (latest stable)
- **Validation**: Ensure consistency across all jobs

### 2. Coverage Calculation Engine
- **Component**: Coverage threshold checker
- **Interface**: Go native tools (`go tool cover`)
- **Logic**: Pure Go arithmetic instead of shell `bc` command
- **Output**: Clear pass/fail with detailed reporting

### 3. Test Execution Framework
- **Component**: Test runner with coverage
- **Interface**: `go test` with standardized flags
- **Configuration**: Consistent flags across local and CI environments
- **Artifacts**: Coverage reports and test results

### 4. Error Reporting System
- **Component**: Failure analysis and reporting
- **Interface**: GitHub Actions annotations and logs
- **Features**: Detailed error messages, coverage summaries, actionable feedback

## Data Models

### Coverage Report Structure
```go
type CoverageReport struct {
    TotalCoverage    float64
    PackageCoverage  map[string]float64
    Threshold        float64
    PassingPackages  []string
    FailingPackages  []string
}
```

### CI Job Configuration
```yaml
JobConfig:
  GoVersion: "1.21"
  CoverageThreshold: 80
  TestFlags: ["-v", "-race", "-coverprofile=coverage.out"]
  BuildFlags: ["-v"]
```

## Error Handling

### 1. Go Version Compatibility
- **Detection**: Check if specified Go version exists
- **Fallback**: Use latest stable version if invalid version specified
- **Validation**: Verify version consistency across jobs

### 2. Coverage Calculation Failures
- **Detection**: Validate coverage file exists and is readable
- **Error Recovery**: Provide clear error messages for missing coverage data
- **Fallback**: Allow manual threshold override for debugging

### 3. Test Environment Differences
- **Detection**: Compare local vs CI test execution
- **Standardization**: Use identical test commands and flags
- **Debugging**: Preserve test artifacts for analysis

### 4. Dependency Issues
- **Detection**: Check for missing system dependencies (bc, awk, etc.)
- **Solution**: Use Go-native solutions instead of shell dependencies
- **Validation**: Test coverage calculation in isolated environment

## Testing Strategy

### 1. CI Configuration Testing
- **Unit Tests**: Validate YAML syntax and job dependencies
- **Integration Tests**: Test complete CI pipeline with sample changes
- **Regression Tests**: Ensure fixes don't break existing functionality

### 2. Coverage Calculation Testing
- **Unit Tests**: Test coverage threshold logic with various scenarios
- **Edge Cases**: Test with 0% coverage, 100% coverage, and boundary values
- **Cross-Platform**: Verify calculation consistency across different runners

### 3. Go Version Compatibility Testing
- **Matrix Testing**: Test with multiple Go versions
- **Dependency Validation**: Ensure all dependencies work with target Go version
- **Build Verification**: Confirm builds work across all target platforms

## Implementation Approach

### Phase 1: Fix Go Version
1. Update all Go version references from "1.24" to "1.21"
2. Verify go.mod compatibility with Go 1.21
3. Test locally with Go 1.21

### Phase 2: Consolidate Coverage Logic
1. Remove separate coverage job
2. Integrate coverage threshold check into test job
3. Use Go-native arithmetic instead of shell commands

### Phase 3: Standardize Test Execution
1. Create consistent test command across all environments
2. Ensure identical flags and options
3. Validate test artifacts and reporting

### Phase 4: Enhance Error Reporting
1. Add detailed failure analysis
2. Provide actionable error messages
3. Include coverage summaries in CI output

## Risk Mitigation

### 1. Breaking Changes
- **Risk**: Go version downgrade might break existing code
- **Mitigation**: Test thoroughly with Go 1.21 before deployment
- **Rollback**: Keep current configuration as backup

### 2. Coverage Regression
- **Risk**: New coverage logic might be more/less strict
- **Mitigation**: Test with known coverage scenarios
- **Validation**: Compare results with current implementation

### 3. CI Pipeline Disruption
- **Risk**: Changes might break CI for other developers
- **Mitigation**: Test changes in feature branch first
- **Communication**: Notify team of CI changes