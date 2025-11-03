#!/bin/bash
# bump-version.sh

if [ $# -eq 0 ]; then
    echo "Usage: $0 <version>"
    echo "Example: $0 0.1.1"
    exit 1
fi

VERSION=$1
CHART_FILE="../documentdb-helm-chart/Chart.yaml"

echo "🔄 Updating Chart.yaml to version $VERSION"

# Update chart version and appVersion
sed -i "s/^version: .*/version: $VERSION/" $CHART_FILE
sed -i "s/^appVersion: .*/appVersion: \"$VERSION\"/" $CHART_FILE

echo "✅ Updated Chart.yaml:"
grep -E "^(version|appVersion):" $CHART_FILE

echo ""
echo "🎯 Next steps:"
echo "   helm package ../documentdb-helm-chart"
echo "   git add $CHART_FILE"
echo "   git commit -m 'Bump version to $VERSION'"
echo "   git tag v$VERSION"