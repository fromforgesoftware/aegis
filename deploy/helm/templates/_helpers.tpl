{{- define "aegis.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "aegis.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name (include "aegis.name" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}

{{- define "aegis.labels" -}}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{ include "aegis.selectorLabels" . }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: forge
{{- end -}}

{{- define "aegis.selectorLabels" -}}
app.kubernetes.io/name: {{ include "aegis.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{- define "aegis.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
{{- default (include "aegis.fullname" .) .Values.serviceAccount.name -}}
{{- else -}}
{{- default "default" .Values.serviceAccount.name -}}
{{- end -}}
{{- end -}}

{{- define "aegis.image" -}}
{{- printf "%s:%s" .Values.image.repository (.Values.image.tag | default .Chart.AppVersion) -}}
{{- end -}}

{{- define "aegis.dbSecretName" -}}
{{- if .Values.database.existingSecret -}}
{{- .Values.database.existingSecret -}}
{{- else -}}
{{- printf "%s-db" (include "aegis.fullname" .) -}}
{{- end -}}
{{- end -}}

{{- define "aegis.cryptoSecretName" -}}
{{- if .Values.crypto.existingSecret -}}
{{- .Values.crypto.existingSecret -}}
{{- else -}}
{{- printf "%s-crypto" (include "aegis.fullname" .) -}}
{{- end -}}
{{- end -}}

{{/* Shared env block for server + migrator. */}}
{{- define "aegis.env" -}}
- name: SVC_NAME
  value: {{ include "aegis.name" . | quote }}
- name: REST_ADDRESS
  value: ":{{ .Values.ports.http }}"
- name: HTTP_ADDRESS
  value: ":{{ .Values.ports.http }}"
- name: GRPC_ADDRESS
  value: ":{{ .Values.ports.grpc }}"
- name: DB_HOST
  value: {{ .Values.database.host | quote }}
- name: DB_PORT
  value: {{ .Values.database.port | quote }}
- name: DB_NAME
  value: {{ .Values.database.name | quote }}
- name: DB_SCHEMA
  value: {{ .Values.database.schema | quote }}
- name: DB_SSL
  value: {{ .Values.database.ssl | quote }}
- name: DB_LOG_LEVEL
  value: {{ .Values.database.logLevel | default "warn" | quote }}
- name: DB_USER
  valueFrom:
    secretKeyRef:
      name: {{ include "aegis.dbSecretName" . }}
      key: DB_USER
- name: DB_PASSWORD
  valueFrom:
    secretKeyRef:
      name: {{ include "aegis.dbSecretName" . }}
      key: DB_PASSWORD
- name: AEGIS_KEY_ENCRYPTION_KEY
  valueFrom:
    secretKeyRef:
      name: {{ include "aegis.cryptoSecretName" . }}
      key: AEGIS_KEY_ENCRYPTION_KEY
{{- if .Values.bootstrap.enabled }}
- name: AEGIS_BOOTSTRAP_REALM
  value: {{ .Values.bootstrap.realm | quote }}
- name: AEGIS_BOOTSTRAP_REALM_DISPLAY_NAME
  value: {{ .Values.bootstrap.realmDisplayName | quote }}
- name: AEGIS_BOOTSTRAP_ADMIN_EMAIL
  value: {{ .Values.bootstrap.adminEmail | quote }}
- name: AEGIS_BOOTSTRAP_ADMIN_PASSWORD
  value: {{ .Values.bootstrap.adminPassword | quote }}
- name: AEGIS_BOOTSTRAP_CLIENT_ID
  value: {{ .Values.bootstrap.clientId | quote }}
- name: AEGIS_BOOTSTRAP_CLIENT_REDIRECT_URIS
  value: {{ .Values.bootstrap.redirectUris | quote }}
{{- end }}
{{- if .Values.gatewaySecret }}
- name: FORGE_GATEWAY_SECRET
  value: {{ .Values.gatewaySecret | quote }}
{{- end }}
{{- end -}}
