{{- define "name" -}}
gardener-extension-backupbucket-hcloud
{{- end -}}

{{- define "labels.app.key" -}}
app.kubernetes.io/name
{{- end -}}
{{- define "labels.app.value" -}}
{{ include "name" . }}
{{- end -}}

{{- define "labels" -}}
{{ include "labels.app.key" . }}: {{ include "labels.app.value" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{- define "image" -}}
  {{- $tag := .Values.image.tag | default .Chart.AppVersion -}}
  {{- if hasPrefix "sha256:" $tag }}
  {{- printf "%s@%s" .Values.image.repository $tag }}
  {{- else }}
  {{- printf "%s:%s" .Values.image.repository $tag }}
  {{- end }}
{{- end -}}

{{- define "leaderelectionid" -}}
{{ include "name" . }}-leader-election
{{- end -}}
