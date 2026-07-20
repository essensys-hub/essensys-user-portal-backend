package admin

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Regression: admin CreateUser must not call Turnstile / siteverify.
func TestCreateUserSourceHasNoTurnstile(t *testing.T) {
	path := filepath.Join("handlers.go")
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lower := strings.ToLower(string(src))
	for _, needle := range []string{"turnstile", "siteverify", "captcha"} {
		if strings.Contains(lower, needle) {
			t.Fatalf("admin handlers must not reference %q", needle)
		}
	}

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, src, 0)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	ast.Inspect(f, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok || fn.Name == nil {
			return true
		}
		if fn.Name.Name == "CreateUser" {
			found = true
		}
		return true
	})
	if !found {
		t.Fatal("CreateUser not found")
	}
}
