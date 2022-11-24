{{/*
Expand the name of the chart.
*/}}
{{- define "script-exporter.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "script-exporter.fullname" -}}
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
Create chart name and version as used by the chart label.
*/}}
{{- define "script-exporter.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "script-exporter.labels" -}}
helm.sh/chart: {{ include "script-exporter.chart" . }}
{{ include "script-exporter.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "script-exporter.selectorLabels" -}}
app.kubernetes.io/name: {{ include "script-exporter.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Additional annotations for Pods
*/}}
{{- define "script-exporter.podAnnotations" -}}
{{- if .Values.podAnnotations }}
{{- toYaml .Values.podAnnotations }}
{{- end }}
{{- end }}

{{/*
Additional labels for Pods
*/}}
{{- define "script-exporter.podLabels" -}}
{{- if .Values.podLabels }}
{{- toYaml .Values.podLabels }}
{{- end }}
{{- end }}

{{/*
Additional annotations for the Service
*/}}
{{- define "script-exporter.serviceAnnotations" -}}
{{- if .Values.service.annotations }}
{{- toYaml .Values.service.annotations }}
{{- end }}
{{- end }}

{{/*
Additional labels for the Service
*/}}
{{- define "script-exporter.serviceLabels" -}}
{{- if .Values.service.labels }}
{{- toYaml .Values.service.labels }}
{{- end }}
{{- end }}

{{/*
Additional labels for the Service Monitor
*/}}
{{- define "script-exporter.serviceMonitorLabels" -}}
{{- if .Values.serviceMonitor.labels }}
{{- toYaml .Values.serviceMonitor.labels }}
{{- end }}
{{- end }}
