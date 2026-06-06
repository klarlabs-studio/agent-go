#!/usr/bin/env bash
# Create a new contrib module with the standard structure.
# Usage: ./scripts/new-contrib.sh <module-name>
# Example: ./scripts/new-contrib.sh pack-foo
set -euo pipefail

NAME="${1:?Usage: $0 <module-name> (e.g., pack-foo, storage-memcached)}"
DIR="contrib/$NAME"
MODULE="go.klarlabs.de/agent/contrib/$NAME"

# Derive Go package name from module name (remove prefix, replace hyphens)
PKG=$(echo "$NAME" | sed 's/^pack-//;s/^storage-//;s/^approval-//;s/-/_/g')

if [ -d "$DIR" ]; then
    echo "Error: $DIR already exists"
    exit 1
fi

echo "Creating contrib module: $NAME"
echo "  Directory: $DIR"
echo "  Package: $PKG"
echo "  Module: $MODULE"
echo

# Create directory
mkdir -p "$DIR"

# go.mod
cat > "$DIR/go.mod" <<GOMOD
module $MODULE

go 1.25.0

require go.klarlabs.de/agent v0.0.0

replace go.klarlabs.de/agent => ../..
GOMOD

# Main source file
if [[ "$NAME" == pack-* ]]; then
    cat > "$DIR/$PKG.go" <<'GOSRC'
// Package PKGNAME provides tools for DESCRIPTION.
package PKGNAME

import (
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// Pack returns the tool pack.
func Pack() *pack.Pack {
	return &pack.Pack{
		Name:        "MODNAME",
		Description: "DESCRIPTION tools",
		Version:     "0.1.0",
		Tools:       tools(),
	}
}

func tools() []tool.Tool {
	return []tool.Tool{
		// Add tools here using tool.NewBuilder("tool_name").
		//     WithDescription("Does something").
		//     WithAnnotations(tool.Annotations{ReadOnly: true}).
		//     WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
		//         // Implementation
		//     }).
		//     MustBuild(),
	}
}
GOSRC
    sed -i '' "s/PKGNAME/$PKG/g;s/MODNAME/$NAME/g;s/DESCRIPTION/TODO/g" "$DIR/$PKG.go"

    # Test file
    cat > "$DIR/pack_test.go" <<'GOTEST'
package PKGNAME

import (
	"testing"

	"go.klarlabs.de/agent/domain/tool"
)

func TestPack_RegistersTools(t *testing.T) {
	p := Pack()
	if p == nil {
		t.Fatal("Pack() returned nil")
	}
	if len(p.Tools) == 0 {
		t.Skip("no tools registered yet")
	}
}

func TestPack_ToolsImplementInterface(t *testing.T) {
	p := Pack()
	for _, tt := range p.Tools {
		var _ tool.Tool = tt
		if tt.Name() == "" {
			t.Error("tool has empty name")
		}
	}
}
GOTEST
    sed -i '' "s/PKGNAME/$PKG/g" "$DIR/pack_test.go"
else
    # Non-pack module (storage, approval, etc.)
    cat > "$DIR/$PKG.go" <<'GOSRC'
// Package PKGNAME provides DESCRIPTION.
package PKGNAME
GOSRC
    sed -i '' "s/PKGNAME/$PKG/g;s/DESCRIPTION/TODO/g" "$DIR/$PKG.go"

    cat > "$DIR/${PKG}_test.go" <<'GOTEST'
package PKGNAME

import "testing"

func TestPlaceholder(t *testing.T) {
	// Add tests here
}
GOTEST
    sed -i '' "s/PKGNAME/$PKG/g" "$DIR/${PKG}_test.go"
fi

# Add to go.work
if ! grep -q "./$DIR" go.work 2>/dev/null; then
    # Insert before the closing )
    sed -i '' "/^)/i\\
\\	./$DIR
" go.work
    echo "Added ./$DIR to go.work"
fi

# Run go mod tidy
(cd "$DIR" && go mod tidy 2>/dev/null || true)

echo
echo "Created $DIR/"
ls -la "$DIR/"
echo
echo "Next steps:"
echo "  1. Edit $DIR/$PKG.go — add your tools or implementation"
echo "  2. Run: cd $DIR && go test ./..."
echo "  3. Run: go work sync"
