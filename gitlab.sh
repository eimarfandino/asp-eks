#!/bin/bash

GITLAB_TOKEN=$1
FULL_PATH="EDSN/portaal-cts"
AFTER_CURSOR=null
ALL_VULNS="[]"

while : ; do
  RESPONSE=$(curl -s 'https://gitlab.com/api/graphql' \
    -H "Authorization: Bearer $GITLAB_TOKEN" \
    -H "Content-Type: application/json" \
    --data-raw @- <<EOF
{
  "operationName": "groupVulnerabilities",
  "variables": {
    "first": 100,
    "after": $AFTER_CURSOR,
    "vetEnabled": true,
    "includeSeverityOverrides": true,
    "fullPath": "$FULL_PATH",
    "sort": "severity_desc",
    "state": ["DETECTED", "CONFIRMED"],
    "dismissalReason": [],
    "hasResolution": false,
    "reportType": [
      "API_FUZZING", "CONTAINER_SCANNING", "COVERAGE_FUZZING",
      "DEPENDENCY_SCANNING", "SECRET_DETECTION", "GENERIC", "DAST"
    ]
  },
  "query": "query groupVulnerabilities(\$fullPath: ID!, \$before: String, \$after: String, \$first: Int = 20, \$last: Int, \$projectId: [ID!], \$severity: [VulnerabilitySeverity!], \$reportType: [VulnerabilityReportType!], \$scanner: [String!], \$scannerId: [VulnerabilitiesScannerID!], \$state: [VulnerabilityState!], \$dismissalReason: [VulnerabilityDismissalReason!], \$identifierName: String, \$sort: VulnerabilitySort, \$hasIssues: Boolean, \$hasResolution: Boolean, \$hasMergeRequest: Boolean, \$hasRemediations: Boolean, \$hasAiResolution: Boolean, \$vetEnabled: Boolean = false, \$clusterAgentId: [ClustersAgentID!], \$owaspTopTen: [VulnerabilityOwaspTop10!], \$includeSeverityOverrides: Boolean = false) { group(fullPath: \$fullPath) { vulnerabilities( before: \$before after: \$after first: \$first last: \$last severity: \$severity reportType: \$reportType scanner: \$scanner scannerId: \$scannerId state: \$state dismissalReason: \$dismissalReason projectId: \$projectId sort: \$sort hasIssues: \$hasIssues hasResolution: \$hasResolution hasMergeRequest: \$hasMergeRequest hasRemediations: \$hasRemediations hasAiResolution: \$hasAiResolution clusterAgentId: \$clusterAgentId owaspTopTen: \$owaspTopTen identifierName: \$identifierName ) { nodes { id title state severity reportType location { __typename ... on VulnerabilityLocationDast { path } ... on VulnerabilityLocationDependencyScanning { file } ... on VulnerabilityLocationSast { file startLine } ... on VulnerabilityLocationSecretDetection { file startLine } } project { nameWithNamespace } } pageInfo { endCursor hasNextPage } } } }"
}
EOF
  )

  # Merge this page's nodes with accumulated list
  PAGE_VULNS=$(echo "$RESPONSE" | jq '.data.group.vulnerabilities.nodes')
  ALL_VULNS=$(jq -s 'add' <(echo "$ALL_VULNS") <(echo "$PAGE_VULNS"))

  # Prepare for next page
  HAS_NEXT=$(echo "$RESPONSE" | jq '.data.group.vulnerabilities.pageInfo.hasNextPage')
  if [ "$HAS_NEXT" == "true" ]; then
    AFTER_CURSOR=$(echo "$RESPONSE" | jq -Rr '.data.group.vulnerabilities.pageInfo.endCursor' <<< "$RESPONSE" | jq -R '.')
  else
    break
  fi
done

# Print counts by severity
echo "$ALL_VULNS" | jq -r '
  group_by(.severity) 
  | map({severity: .[0].severity, count: length}) 
  | sort_by(.severity) 
  | .[] 
  | "\(.severity): \(.count)"'
