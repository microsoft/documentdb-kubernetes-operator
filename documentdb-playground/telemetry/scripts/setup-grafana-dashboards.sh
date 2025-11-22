#!/bin/bash

# Script to automatically create Grafana dashboards for CPU and memory monitoring
# Usage: ./setup-grafana-dashboards.sh [namespace]

set -e

NAMESPACE=${1:-"sales-namespace"}
TEAM=$(echo $NAMESPACE | cut -d'-' -f1)

echo "Setting up Grafana dashboard for $TEAM team in $NAMESPACE..."

# Dashboard JSON configuration
DASHBOARD_JSON=$(cat <<'EOF'
{
  "dashboard": {
    "id": null,
    "title": "TEAM_NAME Workload Monitoring",
    "tags": ["TEAM_NAME", "kubernetes", "monitoring"],
    "timezone": "browser",
    "panels": [
      {
        "id": 1,
        "title": "CPU Usage by Container",
        "type": "timeseries",
        "targets": [
          {
            "expr": "rate(container_cpu_usage_seconds_total{namespace=\"NAMESPACE_NAME\",container!=\"POD\",container!=\"\"}[5m]) * 100",
            "legendFormat": "{{container}} - {{pod}}",
            "refId": "A"
          }
        ],
        "fieldConfig": {
          "defaults": {
            "unit": "percent",
            "min": 0,
            "max": 100
          }
        },
        "gridPos": {
          "h": 8,
          "w": 12,
          "x": 0,
          "y": 0
        }
      },
      {
        "id": 2,
        "title": "Memory Usage by Container",
        "type": "timeseries",
        "targets": [
          {
            "expr": "container_memory_working_set_bytes{namespace=\"NAMESPACE_NAME\",container!=\"POD\",container!=\"\"} / 1024 / 1024",
            "legendFormat": "{{container}} - {{pod}}",
            "refId": "A"
          }
        ],
        "fieldConfig": {
          "defaults": {
            "unit": "decmbytes",
            "min": 0
          }
        },
        "gridPos": {
          "h": 8,
          "w": 12,
          "x": 12,
          "y": 0
        }
      },
      {
        "id": 3,
        "title": "Memory Usage Percentage",
        "type": "timeseries",
        "targets": [
          {
            "expr": "(container_memory_working_set_bytes{namespace=\"NAMESPACE_NAME\",container!=\"POD\",container!=\"\"} / container_spec_memory_limit_bytes{namespace=\"NAMESPACE_NAME\",container!=\"POD\",container!=\"\"}) * 100",
            "legendFormat": "{{container}} - {{pod}}",
            "refId": "A"
          }
        ],
        "fieldConfig": {
          "defaults": {
            "unit": "percent",
            "min": 0,
            "max": 100
          }
        },
        "gridPos": {
          "h": 8,
          "w": 12,
          "x": 0,
          "y": 8
        }
      },
      {
        "id": 4,
        "title": "Pod Count",
        "type": "stat",
        "targets": [
          {
            "expr": "count(container_cpu_usage_seconds_total{namespace=\"NAMESPACE_NAME\",container!=\"POD\",container!=\"\"})",
            "legendFormat": "Running Pods",
            "refId": "A"
          }
        ],
        "fieldConfig": {
          "defaults": {
            "unit": "short"
          }
        },
        "gridPos": {
          "h": 8,
          "w": 12,
          "x": 12,
          "y": 8
        }
      }
    ],
    "time": {
      "from": "now-1h",
      "to": "now"
    },
    "refresh": "30s"
  },
  "overwrite": true
}
EOF
)

# Replace placeholders in the JSON
DASHBOARD_JSON=$(echo "$DASHBOARD_JSON" | sed "s/TEAM_NAME/$TEAM/g" | sed "s/NAMESPACE_NAME/$NAMESPACE/g")

# Function to create dashboard via API
create_dashboard() {
    local grafana_url="http://localhost:${1}"
    local auth="admin:admin123"
    
    echo "Creating dashboard for $TEAM team at $grafana_url..."
    
    # Test connection first
    if ! curl -s --fail -u "$auth" "$grafana_url/api/health" > /dev/null; then
        echo "Error: Cannot connect to Grafana at $grafana_url"
        echo "Make sure port-forward is running: kubectl port-forward -n $NAMESPACE svc/grafana-$TEAM ${1}:3000"
        return 1
    fi
    
    # Create the dashboard
    response=$(curl -s -X POST \
        -H "Content-Type: application/json" \
        -u "$auth" \
        -d "$DASHBOARD_JSON" \
        "$grafana_url/api/dashboards/db")
    
    if echo "$response" | grep -q '"status":"success"'; then
        dashboard_url=$(echo "$response" | jq -r '.url')
        echo "✅ Dashboard created successfully!"
        echo "🔗 Access it at: $grafana_url$dashboard_url"
    else
        echo "❌ Error creating dashboard:"
        echo "$response" | jq '.'
        return 1
    fi
}

# Create dashboards based on namespace
case $NAMESPACE in
    "sales-namespace")
        echo "Setting up Sales team dashboard..."
        create_dashboard 3001
        ;;
    "accounts-namespace")
        echo "Setting up Accounts team dashboard..."
        create_dashboard 3002
        ;;
    *)
        echo "Unknown namespace: $NAMESPACE"
        echo "Supported namespaces: sales-namespace, accounts-namespace"
        exit 1
        ;;
esac

echo ""
echo "Dashboard setup complete! 🎉"
echo ""
echo "To view your dashboard:"
echo "1. Open your browser to the URL shown above"
echo "2. Login with username: admin, password: admin123"
echo "3. The dashboard should be available in your dashboards list"
echo ""
echo "The dashboard includes:"
echo "- CPU Usage by Container"
echo "- Memory Usage by Container"
echo "- Memory Usage Percentage"
echo "- Pod Count"
echo ""
echo "All metrics are filtered to show only workloads in the $NAMESPACE namespace."