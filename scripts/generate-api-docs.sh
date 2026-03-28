#!/usr/bin/env bash
set -euo pipefail

# generate-api-docs.sh
#
# Generates markdown API documentation from godoc for key packages.
# Output is written to docs/api/<package-name>.md with an index at docs/api/index.md.

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DOCS_API_DIR="${REPO_ROOT}/docs/api"

mkdir -p "${DOCS_API_DIR}"

# Key packages to document
PACKAGES=(
    "domain/agent"
    "domain/tool"
    "domain/policy"
    "domain/event"
    "domain/cache"
    "domain/knowledge"
    "domain/pack"
    "domain/ledger"
    "domain/artifact"
    "interfaces/api"
    "application"
)

MODULE="github.com/felixgeelhaar/agent-go"

index_entries=""

for pkg in "${PACKAGES[@]}"; do
    full_pkg="${MODULE}/${pkg}"
    pkg_name="$(basename "${pkg}")"
    pkg_dir="${REPO_ROOT}/${pkg}"
    output_file="${DOCS_API_DIR}/${pkg_name}.md"

    # Skip packages that do not exist in the repo
    if [ ! -d "${pkg_dir}" ]; then
        echo "Skipping ${pkg} (directory not found)"
        continue
    fi

    echo "Generating docs for ${pkg}..."

    # Capture godoc output
    doc_output="$(cd "${REPO_ROOT}" && go doc -all "./${pkg}" 2>/dev/null || true)"

    if [ -z "${doc_output}" ]; then
        echo "  Warning: no documentation found for ${pkg}"
        continue
    fi

    # Extract package description (first paragraph before any type/func)
    pkg_description=""
    in_description=true
    types_section=""
    funcs_section=""
    current_section=""

    while IFS= read -r line; do
        # Detect section boundaries
        if echo "${line}" | grep -qE '^type [A-Z]'; then
            current_section="type"
            types_section="${types_section}
### ${line}
\`\`\`go"
        elif echo "${line}" | grep -qE '^func [A-Z(]'; then
            current_section="func"
            funcs_section="${funcs_section}

### ${line}
"
        elif echo "${line}" | grep -qE '^    ' && [ "${current_section}" = "type" ]; then
            types_section="${types_section}
${line}"
        elif [ "${current_section}" = "type" ] && [ -z "${line}" ]; then
            types_section="${types_section}
\`\`\`
"
            current_section=""
        elif [ "${in_description}" = "true" ] && echo "${line}" | grep -qE '^[A-Z]|^[a-z]|^$'; then
            if echo "${line}" | grep -qE '^(type |func |var |const |TYPES|FUNCTIONS|VARIABLES|CONSTANTS)'; then
                in_description=false
            else
                pkg_description="${pkg_description}${line}
"
            fi
        fi
    done <<< "${doc_output}"

    # Write markdown file with raw godoc output in a clean format
    cat > "${output_file}" <<EOF
# Package \`${pkg_name}\`

**Import path:** \`${full_pkg}\`

## Overview

$(echo "${pkg_description}" | head -20)

## Full API Reference

\`\`\`
${doc_output}
\`\`\`
EOF

    index_entries="${index_entries}
- [${pkg_name}](./${pkg_name}.md) - \`${full_pkg}\`"

    echo "  Written: ${output_file}"
done

# Generate index file
cat > "${DOCS_API_DIR}/index.md" <<EOF
# API Documentation

Generated from source using \`go doc\`.

## Packages
${index_entries}

---

*Generated on $(date -u +"%Y-%m-%d")*
EOF

echo ""
echo "Generated API documentation in ${DOCS_API_DIR}/"
echo "Index: ${DOCS_API_DIR}/index.md"
