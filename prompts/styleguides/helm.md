# Helm Style Guide

Best practices for production-ready Helm charts.

## Chart.yaml

Use apiVersion v2 (Helm 3+). All required fields must be present.

```yaml
apiVersion: v2
name: my-app
description: A Helm chart for My Application
type: application       # application or library
version: 1.2.3          # chart version — increment on chart changes
appVersion: "2.5.0"     # application version — any string, not required to be SemVer
```

`version` follows SemVer: MAJOR for breaking changes, MINOR for new features, PATCH for fixes.
`appVersion` is independent of `version` — do not conflate them.

Pin dependency versions exactly:

```yaml
dependencies:
  - name: postgresql
    version: "12.0.0"   # exact version, no ranges
    repository: "https://charts.bitnami.com/bitnami"
    condition: postgresql.enabled
```

Commit `Chart.lock` alongside `Chart.yaml`.

## values.yaml

Organize hierarchically. Document every key with inline comments.

```yaml
# Image configuration
image:
  registry: docker.io
  repository: myapp/web
  tag: ""               # defaults to .Chart.AppVersion when empty
  pullPolicy: IfNotPresent

replicaCount: 1

service:
  type: ClusterIP
  port: 80
  targetPort: http

# Security defaults — always set these
podSecurityContext:
  runAsNonRoot: true
  runAsUser: 1000
  fsGroup: 1000

securityContext:
  allowPrivilegeEscalation: false
  readOnlyRootFilesystem: true
  capabilities:
    drop:
      - ALL

resources:
  limits:
    cpu: 100m
    memory: 128Mi
  requests:
    cpu: 100m
    memory: 128Mi

autoscaling:
  enabled: false
  minReplicas: 1
  maxReplicas: 10
  targetCPUUtilizationPercentage: 80

# Global values propagate to subcharts
global:
  imageRegistry: ""
  imagePullSecrets: []
```

## values.schema.json

Validate required fields and constrain enums:

```json
{
  "$schema": "https://json-schema.org/draft-07/schema#",
  "type": "object",
  "required": ["image"],
  "properties": {
    "replicaCount": {
      "type": "integer",
      "minimum": 1
    },
    "image": {
      "type": "object",
      "required": ["repository"],
      "properties": {
        "repository": { "type": "string" },
        "tag": { "type": "string" },
        "pullPolicy": {
          "type": "string",
          "enum": ["Always", "IfNotPresent", "Never"]
        }
      }
    }
  }
}
```

## Template Helpers (_helpers.tpl)

Define all reusable logic in `_helpers.tpl`. Never inline repeated expressions in templates.

```yaml
{{- define "my-app.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "my-app.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{- define "my-app.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "my-app.labels" -}}
helm.sh/chart: {{ include "my-app.chart" . }}
{{ include "my-app.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{- define "my-app.selectorLabels" -}}
app.kubernetes.io/name: {{ include "my-app.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{- define "my-app.image" -}}
{{- $registry := .Values.global.imageRegistry | default .Values.image.registry -}}
{{- printf "%s/%s:%s" $registry .Values.image.repository (.Values.image.tag | default .Chart.AppVersion) -}}
{{- end -}}
```

## Naming Conventions

- Resource names: lowercase, hyphens only — `my-app-worker`, not `myAppWorker`
- All names truncated to 63 characters (Kubernetes DNS label limit)
- Template partial names prefixed with chart name: `my-app.fullname`
- Partial template files: underscore prefix `_helpers.tpl`
- Test files: `templates/tests/`

## Labels

Apply `app.kubernetes.io/*` labels on every resource via helpers. Never hardcode label values.

```yaml
metadata:
  name: {{ include "my-app.fullname" . }}
  labels:
    {{- include "my-app.labels" . | nindent 4 }}
spec:
  selector:
    matchLabels:
      {{- include "my-app.selectorLabels" . | nindent 6 }}
  template:
    metadata:
      labels:
        {{- include "my-app.selectorLabels" . | nindent 8 }}
```

## Indentation

YAML is whitespace-sensitive. Wrong indentation produces silently malformed manifests.

Use `nindent N` (adds a leading newline then N spaces) when the `{{- include }}` or `{{- toYaml }}` call is on its own line:

```yaml
# nindent adds newline + 4 spaces — correct for a block under `labels:`
labels:
  {{- include "my-app.labels" . | nindent 4 }}

# toYaml + nindent for multi-key blocks
resources:
  {{- toYaml .Values.resources | nindent 2 }}
```

Use `indent N` (no leading newline) only when the output continues an existing line.

The `{{-` prefix strips whitespace/newlines **before** the action; `-}}` strips **after**. Use `{{-` on lines that would otherwise leave a blank line or unwanted indent:

```yaml
spec:
  {{- if not .Values.autoscaling.enabled }}
  replicas: {{ .Values.replicaCount }}
  {{- end }}
```

N must equal the YAML nesting depth in spaces. Off-by-two errors are a common source of broken renders — verify with `helm template` before committing.

## Templating Patterns

**Quote all string values** to prevent YAML type coercion:

```yaml
value: {{ .Values.someString | quote }}
```

**Conditional resources** — wrap entire file in `{{- if }}`:

```yaml
{{- if .Values.ingress.enabled }}
apiVersion: networking.k8s.io/v1
kind: Ingress
...
{{- end }}
```

**Range over lists**:

```yaml
env:
  {{- range .Values.env }}
  - name: {{ .name | quote }}
    value: {{ .value | quote }}
  {{- end }}
```

**Default values** to prevent nil errors:

```yaml
image: "{{ .Values.image.repository }}:{{ .Values.image.tag | default .Chart.AppVersion }}"
```

## Hooks

Use hooks for DB migrations and setup jobs. Always set a deletion policy to avoid stale hook objects.

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: {{ include "my-app.fullname" . }}-migration
  annotations:
    "helm.sh/hook": pre-install,pre-upgrade
    "helm.sh/hook-weight": "-5"           # lower runs first
    "helm.sh/hook-delete-policy": before-hook-creation,hook-succeeded
spec:
  template:
    spec:
      restartPolicy: Never
      containers:
        - name: migration
          image: {{ include "my-app.image" . }}
          command: ["./migrate"]
```

Hook types: `pre-install`, `post-install`, `pre-upgrade`, `post-upgrade`, `pre-delete`, `post-delete`, `test`.

## Tests

Every chart should have a connectivity test.

```yaml
# templates/tests/test-connection.yaml
apiVersion: v1
kind: Pod
metadata:
  name: "{{ include "my-app.fullname" . }}-test-connection"
  annotations:
    "helm.sh/hook": test
    "helm.sh/hook-delete-policy": before-hook-creation,hook-succeeded
spec:
  restartPolicy: Never
  containers:
    - name: wget
      image: busybox
      command: ["wget"]
      args: ["{{ include "my-app.fullname" . }}:{{ .Values.service.port }}"]
```

Run with: `helm test <release-name>`

## Best Practices

1. Pin dependency versions exactly — no version ranges
2. Commit `Chart.lock`
3. Document every value in `values.yaml` with inline comments
4. Add `values.schema.json` to validate required fields
5. Use `_helpers.tpl` for all repeated logic — never inline
6. Apply `app.kubernetes.io/*` labels on every resource via helpers
7. Always set security context: non-root user, `readOnlyRootFilesystem: true`, drop all capabilities
8. Use `nindent` for block values; match N to YAML nesting depth exactly
9. Quote all string values with `| quote`
10. Gate optional resources with `{{- if .Values.feature.enabled }}`
11. Use `pre-install,pre-upgrade` hooks for DB migrations with `hook-delete-policy`
12. Validate rendering locally: `helm lint` and `helm template` before pushing
