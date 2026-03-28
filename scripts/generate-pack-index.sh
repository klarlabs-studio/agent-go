#!/usr/bin/env bash
set -euo pipefail

# generate-pack-index.sh
#
# Scans all contrib/pack-* modules and generates a JSON index at docs/packs.json.
# Each pack entry includes: name, package, category, tool_count, tested, has_handlers.

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CONTRIB_DIR="${REPO_ROOT}/contrib"
DOCS_DIR="${REPO_ROOT}/docs"
OUTPUT="${DOCS_DIR}/packs.json"

mkdir -p "${DOCS_DIR}"

# Category mapping: pack name suffix -> category
get_category() {
    local name="$1"
    case "${name}" in
        pack-archive|pack-compression)
            echo "Archive & Compression" ;;
        pack-ascii|pack-base64|pack-case|pack-code|pack-diff|pack-hash|pack-markdown|pack-pluralize|pack-regex|pack-similarity|pack-slug|pack-string|pack-template)
            echo "Text & String Processing" ;;
        pack-html|pack-json|pack-xml|pack-yaml)
            echo "Data Formats" ;;
        pack-ast|pack-changelog|pack-docs|pack-license|pack-openapi|pack-semver)
            echo "Code & AST" ;;
        pack-cache|pack-calendar|pack-chunker|pack-collection|pack-color|pack-config|pack-convert|pack-cron|pack-env|pack-math|pack-path|pack-prompt|pack-random|pack-rate|pack-retry|pack-scheduler|pack-stats|pack-testing|pack-time|pack-uuid|pack-validate)
            echo "Utilities" ;;
        pack-api|pack-browser|pack-dns|pack-graphql|pack-grpc|pack-http|pack-ip|pack-network|pack-scrape|pack-sse|pack-url|pack-useragent|pack-websocket)
            echo "Web & Network" ;;
        pack-ci|pack-cloud|pack-database|pack-filesystem|pack-fileops|pack-docker|pack-git|pack-kubernetes|pack-process|pack-shell|pack-sql|pack-ssh|pack-terraform)
            echo "Infrastructure" ;;
        pack-email|pack-messaging|pack-mqtt|pack-notification|pack-slack)
            echo "Messaging & Notifications" ;;
        pack-embeddings|pack-llm|pack-model|pack-rag|pack-vectordb|pack-tokenizer)
            echo "AI & ML" ;;
        pack-audio|pack-chart|pack-image|pack-ocr|pack-pdf|pack-qrcode|pack-spreadsheet|pack-video|pack-visualization)
            echo "Media & Documents" ;;
        pack-copilot|pack-github|pack-jira|pack-linear)
            echo "Integrations" ;;
        pack-crypto|pack-jwt|pack-password|pack-secrets)
            echo "Security & Crypto" ;;
        pack-analytics|pack-dataframe|pack-etl|pack-finance|pack-metrics|pack-monitoring|pack-payments|pack-search)
            echo "Data & Analytics" ;;
        pack-country|pack-geo|pack-phone|pack-emoji)
            echo "Geo & Localization" ;;
        pack-gpio|pack-serial)
            echo "Hardware & IoT" ;;
        pack-crm|pack-creditcard)
            echo "Business & CRM" ;;
        pack-fuzzy|pack-mime|pack-sysinfo)
            echo "Validation & Parsing" ;;
        *)
            echo "Other" ;;
    esac
}

# Build JSON array of packs
pack_count=0
total_tools=0
packs_json=""

for pack_dir in "${CONTRIB_DIR}"/pack-*/; do
    [ -d "${pack_dir}" ] || continue

    pack_name="$(basename "${pack_dir}")"

    # Extract package name from main .go file (strip "pack-" prefix)
    pkg_suffix="${pack_name#pack-}"
    main_go="${pack_dir}${pkg_suffix}.go"

    if [ -f "${main_go}" ]; then
        pkg_name="$(head -20 "${main_go}" | grep '^package ' | head -1 | awk '{print $2}')"
    else
        # Fallback: find any non-test .go file
        first_go="$(find "${pack_dir}" -maxdepth 1 -name '*.go' ! -name '*_test.go' | head -1)"
        if [ -n "${first_go}" ]; then
            pkg_name="$(head -20 "${first_go}" | grep '^package ' | head -1 | awk '{print $2}')"
        else
            pkg_name="${pkg_suffix}"
        fi
    fi

    # Count tools by grepping for MustBuild() or .Build() calls
    tool_count=0
    for gofile in "${pack_dir}"*.go; do
        [ -f "${gofile}" ] || continue
        # Skip test files
        case "${gofile}" in *_test.go) continue ;; esac
        count="$(grep -c 'MustBuild()\|\.Build()' "${gofile}" 2>/dev/null || true)"
        tool_count=$((tool_count + count))
    done

    # Determine category
    category="$(get_category "${pack_name}")"

    # Check if test file exists
    if [ -f "${pack_dir}pack_test.go" ] || ls "${pack_dir}"*_test.go >/dev/null 2>&1; then
        tested="true"
    else
        tested="false"
    fi

    # Check if handlers exist (WithHandler or WithExecutor)
    has_handlers="false"
    for gofile in "${pack_dir}"*.go; do
        [ -f "${gofile}" ] || continue
        case "${gofile}" in *_test.go) continue ;; esac
        if grep -q 'WithHandler\|WithExecutor' "${gofile}" 2>/dev/null; then
            has_handlers="true"
            break
        fi
    done

    # Build JSON entry
    entry="$(printf '    {\n      "name": "%s",\n      "package": "%s",\n      "category": "%s",\n      "tool_count": %d,\n      "tested": %s,\n      "has_handlers": %s\n    }' \
        "${pack_name}" "${pkg_name}" "${category}" "${tool_count}" "${tested}" "${has_handlers}")"

    if [ -n "${packs_json}" ]; then
        packs_json="${packs_json},
${entry}"
    else
        packs_json="${entry}"
    fi

    pack_count=$((pack_count + 1))
    total_tools=$((total_tools + tool_count))
done

# Generate timestamp in ISO 8601 format
timestamp="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"

# Write output JSON
cat > "${OUTPUT}" <<EOF
{
  "generated": "${timestamp}",
  "count": ${pack_count},
  "total_tools": ${total_tools},
  "packs": [
${packs_json}
  ]
}
EOF

echo "Generated pack index: ${OUTPUT}"
echo "  Packs: ${pack_count}"
echo "  Total tools: ${total_tools}"
