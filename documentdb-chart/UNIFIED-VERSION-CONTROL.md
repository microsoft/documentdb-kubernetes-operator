# How documentDbVersion Controls Each Component's Version

## 🎯 **Unified Version Control via values.yaml**

### **Option 1: Use documentDbVersion (Recommended)**
```yaml
# values.yaml
documentDbVersion: "0.1.1"

# Results in:
# - Operator image: ghcr.io/microsoft/documentdb-kubernetes-operator/operator:0.1.1
# - Sidecar Injector image: ghcr.io/microsoft/documentdb-kubernetes-operator/sidecar:0.1.1
# - Environment variable: DOCUMENTDB_VERSION=0.1.1 (in both containers)
```

### **Option 2: Use individual tags (Fallback)**
```yaml
# values.yaml
# documentDbVersion: ""  # Not set
image:
  documentdbk8soperator:
    tag: preview
  sidecarinjector:
    tag: preview

# Results in:
# - Operator image: ghcr.io/microsoft/documentdb-kubernetes-operator/operator:preview
# - Sidecar Injector image: ghcr.io/microsoft/documentdb-kubernetes-operator/sidecar:preview
# - No DOCUMENTDB_VERSION environment variable
```

### **Option 3: Mixed approach**
```yaml
# values.yaml
documentDbVersion: "0.1.1"  # Override for image tags
image:
  documentdbk8soperator:
    tag: preview  # Ignored when documentDbVersion is set
  sidecarinjector:
    tag: preview  # Ignored when documentDbVersion is set

# Results in:
# - Uses documentDbVersion for both image tags
# - Sets DOCUMENTDB_VERSION=0.1.1 environment variable
```

## 🔧 **Template Logic**

### **Image Tag Resolution:**
```yaml
# Both containers use this pattern:
image: "{{ .Values.image.COMPONENT.repository }}:{{ .Values.documentDbVersion | default .Values.image.COMPONENT.tag }}"

# Priority: documentDbVersion > individual tag
```

### **Environment Variable:**
```yaml
# Both containers get this environment variable when documentDbVersion is set:
{{- if .Values.documentDbVersion }}
- name: DOCUMENTDB_VERSION
  value: "{{ .Values.documentDbVersion }}"
{{- end }}
```

## 🚀 **Benefits:**

1. **Single Source of Truth**: One version controls both operator and sidecar injector
2. **Environment Consistency**: Both containers have the same DOCUMENTDB_VERSION
3. **Backward Compatibility**: Falls back to individual tags when documentDbVersion is not set
4. **Release Management**: Easy to upgrade both components together

## 📋 **Component Details:**

### **DocumentDB Operator Container:**
- **Purpose**: Main controller that manages DocumentDB custom resources
- **Image**: `ghcr.io/microsoft/documentdb-kubernetes-operator/operator`
- **Uses DOCUMENTDB_VERSION**: For determining DocumentDB instance image tags

### **Sidecar Injector Container:**
- **Purpose**: CNPG plugin that injects DocumentDB sidecars into PostgreSQL pods
- **Image**: `ghcr.io/microsoft/documentdb-kubernetes-operator/sidecar`
- **Uses DOCUMENTDB_VERSION**: For version consistency and plugin functionality

## 📋 **Usage Example:**

```bash
# Deploy with unified version
helm upgrade documentdb-operator ./documentdb-chart \
  --set documentDbVersion=0.1.1

# Deploy with individual tags (fallback)
helm upgrade documentdb-operator ./documentdb-chart \
  --set image.documentdbk8soperator.tag=preview \
  --set image.sidecarinjector.tag=preview
```