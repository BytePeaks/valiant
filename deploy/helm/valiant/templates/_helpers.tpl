{{/*
Expand the name of the chart.
*/}}
{{- define "valiant.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "valiant.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "valiant.labels" -}}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{ include "valiant.selectorLabels" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: valiant
{{- end }}

{{/*
Selector labels
*/}}
{{- define "valiant.selectorLabels" -}}
app.kubernetes.io/name: {{ include "valiant.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Resolve the DATABASE_URL for the Secret.
Priority: database.url (explicit) > auto-construct from postgresql values.
*/}}
{{- define "valiant.databaseUrl" -}}
{{- if .Values.database.url }}
{{- .Values.database.url }}
{{- else if .Values.postgresql.enabled }}
{{- printf "postgres://valiant:%s@%s-postgres.%s.svc.cluster.local:5432/valiant?sslmode=disable" .Values.postgresql.password (include "valiant.fullname" .) .Release.Namespace }}
{{- else }}
{{- fail "database.url must be set when postgresql.enabled is false" }}
{{- end }}
{{- end }}
