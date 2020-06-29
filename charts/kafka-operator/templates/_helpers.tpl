{{/* vim: set filetype=mustache: */}}
{{/*
Expand the name of the chart.
*/}}
{{- define "kafka-operator.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "kafka-operator.fullname" -}}
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

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "kafka-operator.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Compute operator deployment serviceAccountName key
*/}}
{{- define "operator.serviceAccountName" -}}
{{- if .Values.operator.serviceAccount.create -}}
{{ default "default" .Values.operator.serviceAccount.name }}
{{- else -}}
{{- printf "%s" "default" }}
{{- end -}}
{{- end -}}

{{/*
Compute operator prometheus metrics auth proxy service account
*/}}
{{- define "operator.metricsAuthProxy.serviceAccountName" -}}
{{- if .Values.prometheusMetrics.authProxy.serviceAccount.create -}}
{{ default "default" .Values.prometheusMetrics.authProxy.serviceAccount.name }}
{{- else -}}
{{- printf "%s" "default" }}
{{- end -}}
{{- end -}}
