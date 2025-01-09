package util

import (
	"errors"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func FindTypeSource(filePath string, typePrefixImportName string, typeName string) (string, error) {
	// Parse the target file
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.AllErrors)
	if err != nil {
		return "", fmt.Errorf("failed to parse file: %w", err)
	}

	// Find the package path and the package name of the target file
	pkgPath := filepath.Dir(filePath)

	if typePrefixImportName == "" {
		// Find the type definition
		typeSource, err := findTypeInFile(node, typeName)
		if err != nil {
			return "", err
		}
		if typeSource != "" {
			return typeSource, nil
		}

		// Find the type in other files in the same package
		typeSource, err = findTypeInPackage(pkgPath, typeName)
		if err != nil {
			return "", err
		}
		if typeSource != "" {
			return typeSource, nil
		}
	} else {
		importFullName, ok := findImportPath(typePrefixImportName, node.Imports)
		if !ok {
			return "", fmt.Errorf("package %s not found", typePrefixImportName)
		}

		pkgPath, err := resolveImportPath(pkgPath, importFullName)
		if err != nil {
			return "", fmt.Errorf("failed to resolve import path: %w", err)
		}

		if isStandardLibraryPackage(pkgPath) {
			return "", nil
		}

		typeSource, err := findTypeInPackage(pkgPath, typeName)
		if err != nil {
			return "", err
		}
		return typeSource, nil
	}

	return "", nil
}

func findImportPath(pkgShortName string, imports []*ast.ImportSpec) (string, bool) {
	// need to remove version in path, for example, github.com/volatiletech/null/v9
	re := regexp.MustCompile(`/v\d+$`)

	for _, imp := range imports {
		importPath := strings.Trim(imp.Path.Value, `"`)

		// check if the import has an alias
		if imp.Name != nil {
			if imp.Name.Name == pkgShortName {
				return importPath, true
			}
		} else {
			// check if the import path ends with the package name
			path := re.ReplaceAllString(importPath, "")
			parts := strings.Split(path, "/")
			if parts[len(parts)-1] == pkgShortName {
				return importPath, true
			}
		}
	}
	return "", false
}

func isStandardLibraryPackage(pkgPath string) bool {
	goSrc := build.Default.GOROOT

	return strings.HasPrefix(pkgPath, goSrc)
}

func resolveImportPath(basePath, importPath string) (string, error) {
	pkg, err := build.Import(importPath, basePath, build.FindOnly)
	if err != nil {
		return "", err
	}
	return pkg.Dir, nil
}

func findTypeInFile(node *ast.File, typeName string) (string, error) {
	var typeDecl *ast.TypeSpec
	ast.Inspect(node, func(n ast.Node) bool {
		if ts, ok := n.(*ast.TypeSpec); ok && ts.Name.Name == typeName {
			typeDecl = ts
			return false
		}
		return true
	})

	if typeDecl == nil {
		return "", nil
	}

	return formatTypeDeclaration(node, typeName, typeDecl)
}

func formatTypeDeclaration(node *ast.File, typeName string, typeDecl *ast.TypeSpec) (string, error) {
	var buf strings.Builder

	buf.WriteString("type ")

	if err := formatNode(&buf, typeDecl); err != nil {
		return "", fmt.Errorf("failed to format type: %w", err)
	}

	return buf.String(), nil
}

func findTypeInPackage(pkgPath, typeName string) (string, error) {
	var result string

	// Walk through all Go files in the package
	err := filepath.Walk(pkgPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories, non-Go files and test files
		if info.IsDir() || filepath.Ext(info.Name()) != ".go" || strings.HasSuffix(info.Name(), "_test.go") {
			return nil
		}

		// Parse the file
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, path, nil, parser.AllErrors)
		if err != nil {
			return err
		}

		// Check if the type exists in the file
		typeSource, err := findTypeInFile(node, typeName)
		if err != nil {
			return err
		}

		if typeSource != "" {
			result = typeSource
			return filepath.SkipDir // Skip remaining files
		}

		return nil
	})

	if err != nil && !errors.Is(err, filepath.SkipDir) {
		return "", fmt.Errorf("failed to walk package: %w", err)
	}

	return result, nil
}

func formatNode(buf *strings.Builder, node ast.Node) error {
	return printer.Fprint(buf, token.NewFileSet(), node)
}
