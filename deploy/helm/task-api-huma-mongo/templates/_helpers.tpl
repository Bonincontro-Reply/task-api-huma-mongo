{{- define "task-api-huma-mongo.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "task-api-huma-mongo.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := include "task-api-huma-mongo.name" . -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{- define "task-api-huma-mongo.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" -}}
{{- end -}}

{{- define "task-api-huma-mongo.labels" -}}
helm.sh/chart: {{ include "task-api-huma-mongo.chart" . }}
{{ include "task-api-huma-mongo.selectorLabels" . }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{- define "task-api-huma-mongo.selectorLabels" -}}
app.kubernetes.io/name: {{ include "task-api-huma-mongo.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{- define "task-api-huma-mongo.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
{{- default (include "task-api-huma-mongo.fullname" .) .Values.serviceAccount.name -}}
{{- else -}}
{{- default "default" .Values.serviceAccount.name -}}
{{- end -}}
{{- end -}}

{{- define "task-api-huma-mongo.apiName" -}}
{{- printf "%s-api" (include "task-api-huma-mongo.fullname" .) -}}
{{- end -}}

{{- define "task-api-huma-mongo.frontendName" -}}
{{- printf "%s-frontend" (include "task-api-huma-mongo.fullname" .) -}}
{{- end -}}

{{- define "task-api-huma-mongo.mongodbName" -}}
{{- printf "%s-mongodb" (include "task-api-huma-mongo.fullname" .) -}}
{{- end -}}

{{- define "task-api-huma-mongo.mongodbPvcName" -}}
{{- printf "%s-mongodb" (include "task-api-huma-mongo.fullname" .) -}}
{{- end -}}

{{- define "task-api-huma-mongo.mongodbUri" -}}
{{- if and (not .Values.mongodb.enabled) (not .Values.api.env.mongodb.uri) -}}
{{- fail "api.env.mongodb.uri is required when mongodb.enabled is false" -}}
{{- end -}}
{{- $uri := .Values.api.env.mongodb.uri -}}
{{- if $uri -}}
{{- $uri -}}
{{- else -}}
{{- printf "mongodb://%s:%d" (include "task-api-huma-mongo.mongodbName" .) (.Values.mongodb.service.port | int) -}}
{{- end -}}
{{- end -}}
