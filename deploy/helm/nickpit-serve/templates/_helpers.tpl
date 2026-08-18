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
{{- $tag := required "image.tag is required: --set image.tag=v0.0.x (no default; the deploy repo pins it)" .Values.image.tag -}}
{{- printf "%s:%s" .Values.image.repository $tag -}}
{{- end -}}

{{/*
The GitLab instance is deliberately not defaulted; every install must name it
explicitly so a chart copied to another environment cannot silently point at
the wrong GitLab.
*/}}
{{- define "nickpit-serve.gitlabBaseURL" -}}
{{- required "serve.gitlabBaseURL is required: --set serve.gitlabBaseURL=https://gitlab.example.com" .Values.serve.gitlabBaseURL -}}
{{- end -}}

{{/*
Renders the `config` volume shared by the daemon Deployment and the
comment-template hook Job, so both read the same server.yaml. With
groupsSecretKey the directory is projected from the ConfigMap plus the Secret's
group inventory, so server.yaml's groups_file sits next to it at
/etc/nickpit/groups.yaml.
*/}}
{{- define "nickpit-serve.configVolume" -}}
- name: config
  {{- if .Values.serve.groupsSecretKey }}
  projected:
    sources:
      - configMap:
          name: {{ include "nickpit-serve.fullname" . }}
          items:
            {{- if .Values.config.nickpitYaml }}
            - key: nickpit.yaml
              path: nickpit.yaml
            {{- end }}
            - key: server.yaml
              path: server.yaml
      - secret:
          name: {{ include "nickpit-serve.secretName" . }}
          items:
            - key: {{ .Values.serve.groupsSecretKey }}
              path: groups.yaml
  {{- else }}
  configMap:
    name: {{ include "nickpit-serve.fullname" . }}
    items:
      {{- if .Values.config.nickpitYaml }}
      - key: nickpit.yaml
        path: nickpit.yaml
      {{- end }}
      - key: server.yaml
        path: server.yaml
  {{- end }}
{{- end -}}

{{/*
Renders server.yaml (the serve daemon config). Groups come from the Secret key
serve.groupsSecretKey, mounted at /etc/nickpit/groups.yaml and referenced via
groups_file (default), and/or from inline serve.groups entries whose
credentials are emitted as ${ENV} placeholders resolved at runtime from the
injected Secret — either way no secret text lands in the ConfigMap. Each inline
group uses a GitLab signing token (signingTokenEnv, HMAC verification,
recommended) or the legacy plaintext secret token (webhookSecretEnv) — set
exactly one per group.
*/}}
{{- define "nickpit-serve.serverYaml" -}}
{{- if not (or .Values.serve.groupsSecretKey .Values.serve.groups) -}}
{{- fail "configure a group source: serve.groupsSecretKey (groups from the Secret) or serve.groups (inline)" -}}
{{- end -}}
listen: {{ .Values.serve.listen | quote }}
log_dir: {{ .Values.serve.logDir | quote }}
{{- if and .Values.persistence.enabled .Values.serve.stateDir }}
# PVC root can be group-writable through fsGroup. Keep journal files in a
# process-owned private child created by the daemon with mode 0700.
state_dir: {{ printf "%s/journal" (trimSuffix "/" (clean .Values.serve.stateDir)) | quote }}
{{- else }}
state_dir: {{ .Values.serve.stateDir | quote }}
{{- end }}
review_concurrency: {{ .Values.serve.reviewConcurrency }}
shutdown_grace: {{ .Values.serve.shutdownGrace | quote }}
gitlab_base_url: {{ include "nickpit-serve.gitlabBaseURL" . | quote }}
topic: {{ .Values.serve.topic | quote }}
trigger_emoji: {{ .Values.serve.triggerEmoji | quote }}
start_emoji: {{ .Values.serve.startEmoji | quote }}
command_keyword: {{ .Values.serve.commandKeyword | quote }}
ack_emoji: {{ .Values.serve.ackEmoji | quote }}
abort_emoji: {{ .Values.serve.abortEmoji | quote }}
{{- if ne .Values.serve.doneEmoji "white_check_mark" }}
done_emoji: {{ .Values.serve.doneEmoji | quote }}
{{- end }}
{{- if ne .Values.serve.failEmoji "x" }}
fail_emoji: {{ .Values.serve.failEmoji | quote }}
{{- end }}
{{- if .Values.serve.groupsSecretKey }}
groups_file: "/etc/nickpit/groups.yaml"
{{- end }}
{{- if .Values.serve.groups }}
groups:
{{- range .Values.serve.groups }}
  - path: {{ .path | quote }}
    token: {{ printf "${%s}" .tokenEnv | quote }}
    {{- if .signingTokenEnv }}
    signing_token: {{ printf "${%s}" .signingTokenEnv | quote }}
    {{- else if .webhookSecretEnv }}
    webhook_secret: {{ printf "${%s}" .webhookSecretEnv | quote }}
    {{- else }}
    {{- fail (printf "serve.groups entry %q needs signingTokenEnv or webhookSecretEnv" .path) }}
    {{- end }}
{{- end }}
{{- end }}
review:
  extra_args: {{ toYaml .Values.serve.review.extraArgs | nindent 4 }}
chat:
  enabled: {{ .Values.serve.chat.enabled }}
  opt_in: {{ .Values.serve.chat.optIn }}
  {{- if ne .Values.serve.chat.muteEmoji "mute" }}
  mute_emoji: {{ .Values.serve.chat.muteEmoji | quote }}
  {{- end }}
  skip_phrases: {{ toYaml .Values.serve.chat.skipPhrases | nindent 4 }}
  max_concurrent: {{ .Values.serve.chat.maxConcurrent }}
  {{- if ne .Values.serve.chat.extraArgs nil }}
  extra_args: {{ toYaml .Values.serve.chat.extraArgs | nindent 4 }}
  {{- end }}
{{- if .Values.serve.loki.url }}
loki:
  url: {{ .Values.serve.loki.url | quote }}
  {{- if .Values.serve.loki.tenantIdEnv }}
  tenant_id: {{ printf "${%s}" .Values.serve.loki.tenantIdEnv | quote }}
  {{- end }}
  {{- if .Values.serve.loki.basicAuthUserEnv }}
  basic_auth_user: {{ printf "${%s}" .Values.serve.loki.basicAuthUserEnv | quote }}
  {{- end }}
  {{- if .Values.serve.loki.basicAuthPassEnv }}
  basic_auth_pass: {{ printf "${%s}" .Values.serve.loki.basicAuthPassEnv | quote }}
  {{- end }}
  {{- with .Values.serve.loki.labels }}
  labels: {{ toYaml . | nindent 4 }}
  {{- end }}
  batch_wait: {{ .Values.serve.loki.batchWait | quote }}
  batch_max_lines: {{ .Values.serve.loki.batchMaxLines }}
  timeout: {{ .Values.serve.loki.timeout | quote }}
  buffer_lines: {{ .Values.serve.loki.bufferLines }}
  gzip: {{ .Values.serve.loki.gzip }}
{{- end }}
{{- end -}}
