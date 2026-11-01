package crosschain

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// TestLocalReplayAPIBoundaryV067 prevents accidental re-export of the
// structurally checked, non-authorized low-level storage reservation method.
// Package-internal callers remain trusted code; this does NOT authenticate
// checkpoint provenance or provide a network-facing privilege boundary.
func TestLocalReplayAPIBoundaryV067(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "local_replay_reservation_v067.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	privateWriter := 0
	publicInspector := 0
	for _, declaration := range file.Decls {
		fn, ok := declaration.(*ast.FuncDecl)
		if !ok || fn.Recv == nil || len(fn.Recv.List) != 1 {
			continue
		}
		receiver := fn.Recv.List[0].Type
		if ptr, ok := receiver.(*ast.StarExpr); ok {
			receiver = ptr.X
		}
		ident, ok := receiver.(*ast.Ident)
		if !ok || ident.Name != "LocalReplayStoreV067" {
			continue
		}
		switch fn.Name.Name {
		case "reserveProofV067":
			privateWriter++
		case "InspectProofV067":
			publicInspector++
		default:
			if fn.Name.IsExported() {
				t.Fatalf("unexpected exported LocalReplayStoreV067 method %q", fn.Name.Name)
			}
		}
	}
	if privateWriter != 1 || publicInspector != 1 {
		t.Fatalf("expected exactly one private writer and public inspector; got %d and %d", privateWriter, publicInspector)
	}
}
