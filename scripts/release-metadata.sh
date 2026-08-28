#!/usr/bin/env bash
set -Eeuo pipefail

output_dir=$1
binary=$2
version=$3
go_command=${GO:-go}
mkdir -p "$output_dir"

checksum_file="$output_dir/$(basename "$binary").sha256"
sbom_file="$output_dir/$(basename "$binary").spdx.json"
shasum -a 256 "$binary" > "$checksum_file"

module_graph=$($go_command list -m -mod=readonly -json all | shasum -a 256 | awk '{print $1}')
commit=$(git rev-parse HEAD)
clean_tree=true
git diff --quiet && git diff --cached --quiet || clean_tree=false
compiler=$($go_command version | awk '{print $3}')
printf '{\n  "spdxVersion": "SPDX-2.3",\n  "dataLicense": "CC0-1.0",\n  "SPDXID": "SPDXRef-DOCUMENT",\n  "name": "promptline-%s",\n  "documentNamespace": "https://github.com/dyne/promptline/releases/%s/%s",\n  "creationInfo": {"creators": ["Tool: promptline-release-metadata"], "created": "1970-01-01T00:00:00Z"},\n  "packages": [{"SPDXID": "SPDXRef-Package", "name": "promptline", "versionInfo": "%s", "downloadLocation": "NOASSERTION", "filesAnalyzed": false, "externalRefs": [{"referenceCategory": "OTHER", "referenceType": "module-graph-sha256", "referenceLocator": "%s"}]}],\n  "annotations": [{"annotationType": "OTHER", "annotator": "Tool: promptline-release-metadata", "annotationDate": "1970-01-01T00:00:00Z", "comment": "commit=%s clean_tree=%s compiler=%s build_flags=-trimpath,-s,-w"}]\n}\n' "$version" "$version" "$(basename "$binary")" "$version" "$module_graph" "$commit" "$clean_tree" "$compiler" > "$sbom_file"
