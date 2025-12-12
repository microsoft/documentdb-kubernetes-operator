# DocumentDB Operator Version Management

## 🎯 **Unified Version Control Strategy**

The DocumentDB operator uses a **three-tier priority system** for version management, providing flexibility while maintaining simplicity.

### **Priority Order (Highest to Lowest):**
1. **Component-specific tag** (individual image tag overrides)
2. **Global documentDbVersion** (unified version for both components)  
3. **Chart.AppVersion** (default version from Chart.yaml)

## 📋 **Configuration Options**

### **Option 1: Use Chart Default (Simplest)**
```yaml
# Chart.yaml automatically provides default version
# No configuration needed in values.yaml

# Results in:
# - Operator image: ghcr.io/microsoft/documentdb-kubernetes-operator/operator:0.1.0
# - Sidecar Injector image: ghcr.io/microsoft/documentdb-kubernetes-operator/sidecar:0.1.0
# - Environment variable: DOCUMENTDB_VERSION=0.1.0 (in both containers)
```

### **Option 2: Global Version Override (Recommended)**
```yaml
# values.yaml
documentDbVersion: "0.1.1"

# Results in:
# - Operator image: ghcr.io/microsoft/documentdb-kubernetes-operator/operator:0.1.1
# - Sidecar Injector image: ghcr.io/microsoft/documentdb-kubernetes-operator/sidecar:0.1.1
# - Environment variable: DOCUMENTDB_VERSION=0.1.1 (in both containers)
```

### **Option 3: Component-Specific Tags (Advanced)**
```yaml
# values.yaml
image:
  documentdbk8soperator:
    tag: "preview"
  sidecarinjector:
    tag: "v0.2.0-rc1"

# Results in:
# - Operator image: ghcr.io/microsoft/documentdb-kubernetes-operator/operator:preview
# - Sidecar Injector image: ghcr.io/microsoft/documentdb-kubernetes-operator/sidecar:v0.2.0-rc1
# - Environment variable: DOCUMENTDB_VERSION=0.1.0 (Chart.AppVersion fallback)
```

### **Option 4: Mixed Configuration**
```yaml
# values.yaml
documentDbVersion: "0.1.1"
image:
  documentdbk8soperator:
    tag: "preview"  # This overrides documentDbVersion for operator
  # sidecarinjector has no tag, so uses documentDbVersion

# Results in:
# - Operator image: ghcr.io/microsoft/documentdb-kubernetes-operator/operator:preview
# - Sidecar Injector image: ghcr.io/microsoft/documentdb-kubernetes-operator/sidecar:0.1.1
# - Environment variable: DOCUMENTDB_VERSION=0.1.1
```

## 🔧 **Template Implementation**

### **Image Tag Resolution:**
```yaml
# Both containers use this priority logic:
image: "{{ .Values.image.COMPONENT.repository }}:{{ .Values.image.COMPONENT.tag | default .Values.documentDbVersion | default .Chart.AppVersion }}"

# Priority: Component tag > Global documentDbVersion > Chart.AppVersion
```

### **Environment Variable Injection:**
```yaml
# Both containers get DOCUMENTDB_VERSION environment variable:
- name: DOCUMENTDB_VERSION
  value: "{{ .Values.documentDbVersion | default .Chart.AppVersion }}"

# Uses: documentDbVersion OR Chart.AppVersion fallback
```

## 🚀 **Key Benefits:**

1. **Simple Defaults**: Chart.AppVersion provides out-of-the-box version without configuration
2. **Flexible Overrides**: Global documentDbVersion for unified upgrades
3. **Component Control**: Individual tags for mixed-version testing
4. **Environment Consistency**: DOCUMENTDB_VERSION available in both containers
5. **Standard Helm Patterns**: Uses built-in Chart.AppVersion convention

## 📋 **Component Details:**

### **DocumentDB Operator Container:**
- **Purpose**: Main controller managing DocumentDB custom resources
- **Image**: `ghcr.io/microsoft/documentdb-kubernetes-operator/operator`
- **Version Source**: Component tag → documentDbVersion → Chart.AppVersion
- **Uses DOCUMENTDB_VERSION**: For DocumentDB instance image selection

### **Sidecar Injector Container:**
- **Purpose**: CNPG plugin injecting DocumentDB sidecars into PostgreSQL pods  
- **Image**: `ghcr.io/microsoft/documentdb-kubernetes-operator/sidecar`
- **Version Source**: Component tag → documentDbVersion → Chart.AppVersion
- **Uses DOCUMENTDB_VERSION**: For version consistency and plugin functionality

## � **File Structure:**

### **Chart.yaml:**
```yaml
# Default version for all components
appVersion: "0.1.0"  # DocumentDB version (not chart version)
```

### **values.yaml:**
```yaml
# Optional global override
documentDbVersion: ""

# Optional component-specific overrides  
image:
  documentdbk8soperator:
    tag: ""
  sidecarinjector:
    tag: ""
```

## 🔧 **Version Management Workflow:**

### **Development:**
```bash
# Use component tags for testing
helm upgrade documentdb-operator ./operator/documentdb-helm-chart \
  --set image.documentdbk8soperator.tag=preview \
  --set image.sidecarinjector.tag=dev-branch
```

### **Staging:**
```bash
# Use global version for consistency
helm upgrade documentdb-operator ./operator/documentdb-helm-chart \
  --set documentDbVersion=0.1.1
```

### **Production:**
```bash
# Default versions from Chart.yaml
helm upgrade documentdb-operator ./operator/documentdb-helm-chart
```

**Adding Version Constraint:**
```bash
helm upgrade documentdb-operator ./operator/documentdb-helm-chart \
  --set documentDbVersion=0.1.3
```