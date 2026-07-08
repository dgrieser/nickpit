{{- define "nickpit-serve.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "nickpit-serve.fullname" -}}
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

{{- define "nickpit-serve.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "nickpit-serve.labels" -}}
helm.sh/chart: {{ include "nickpit-serve.chart" . }}
{{ include "nickpit-serve.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{- define "nickpit-serve.selectorLabels" -}}
app.kubernetes.io/name: {{ include "nickpit-serve.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{- define "nickpit-serve.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
{{- default (include "nickpit-serve.fullname" .) .Values.serviceAccount.name -}}
{{- else -}}
{{- default "default" .Values.serviceAccount.name -}}
{{- end -}}
{{- end -}}

{{- define "nickpit-serve.secretName" -}}
{{- if .Values.existingSecret -}}
{{- .Values.existingSecret -}}
{{- else -}}
{{- include "nickpit-serve.fullname" . -}}
{{- end -}}
{{- end -}}

{{- define "nickpit-serve.imageRef" -}}
{{- $tag := .Values.image.tag | default .Chart.AppVersion -}}
{{- printf "%s:%s" .Values.image.repository $tag -}}
{{- end -}}

{{/*
Renders server.yaml (the serve daemon config). Group token/webhook_secret are
emitted as ${ENV} placeholders resolved at runtime from the injected Secret; no
secret text ever lands in the ConfigMap.
*/}}
{{- define "nickpit-serve.serverYaml" -}}
listen: {{ .Values.serve.listen | quote }}
log_dir: {{ .Values.serve.logDir | quote }}
review_concurrency: {{ .Values.serve.reviewConcurrency }}
shutdown_grace: {{ .Values.serve.shutdownGrace | quote }}
gitlab_base_url: {{ .Values.serve.gitlabBaseURL | quote }}
topic: {{ .Values.serve.topic | quote }}
trigger_emoji: {{ .Values.serve.triggerEmoji | quote }}
start_emoji: {{ .Values.serve.startEmoji | quote }}
command_keyword: {{ .Values.serve.commandKeyword | quote }}
ack_emoji: {{ .Values.serve.ackEmoji | quote }}
groups:
{{- range .Values.serve.groups }}
  - path: {{ .path | quote }}
    token: {{ printf "${%s}" .tokenEnv | quote }}
    webhook_secret: {{ printf "${%s}" .webhookSecretEnv | quote }}
{{- end }}
review:
  extra_args: {{ toYaml .Values.serve.review.extraArgs | nindent 4 }}
{{- end -}}
