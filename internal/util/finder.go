package util

import (
	"bytes"
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
			return "", nil
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

func FindFunctionSource(filePath string, packageName string, typeName string, funcName string) (string, error) {
	// Parse the target file
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.AllErrors)
	if err != nil {
		return "", fmt.Errorf("failed to parse file: %w", err)
	}

	// Find the package path and the package name of the target file
	pkgPath := filepath.Dir(filePath)

	if packageName == "" {
		// Find the function or method definition in the file
		funcSource, err := findFunctionOrMethodInFile(node, typeName, funcName)
		if err != nil {
			return "", err
		}
		if funcSource != "" {
			return funcSource, nil
		}

		// Find the function or method in other files in the same package
		funcSource, err = findFunctionOrMethodInPackage(pkgPath, typeName, funcName)
		if err != nil {
			return "", err
		}
		if funcSource != "" {
			return funcSource, nil
		}
	} else {
		importFullName, ok := findImportPath(packageName, node.Imports)
		if !ok {
			return "", fmt.Errorf("package %s not found", packageName)
		}

		pkgPath, err := resolveImportPath(pkgPath, importFullName)
		if err != nil {
			return "", fmt.Errorf("failed to resolve import path: %w", err)
		}

		if isStandardLibraryPackage(pkgPath) {
			return "", nil
		}

		funcSource, err := findFunctionOrMethodInPackage(pkgPath, typeName, funcName)
		if err != nil {
			return "", err
		}
		return funcSource, nil
	}

	return "", nil
}

// Helper to find function or method in a single file
func findFunctionOrMethodInFile(node *ast.File, typeName, funcName string) (string, error) {
	for _, decl := range node.Decls {
		if funcDecl, ok := decl.(*ast.FuncDecl); ok {
			// Check if it's a method
			if funcDecl.Recv != nil {
				for _, field := range funcDecl.Recv.List {
					if starExpr, ok := field.Type.(*ast.StarExpr); ok {
						// Pointer receiver (e.g., *TypeName)
						if ident, ok := starExpr.X.(*ast.Ident); ok && ident.Name == typeName {
							if funcDecl.Name.Name == funcName {
								return extractSourceCode(funcDecl), nil
							}
						}
					} else if ident, ok := field.Type.(*ast.Ident); ok && ident.Name == typeName {
						// Value receiver (e.g., TypeName)
						if funcDecl.Name.Name == funcName {
							return extractSourceCode(funcDecl), nil
						}
					}
				}
			} else if typeName == "" && funcDecl.Name.Name == funcName {
				// Top-level function
				return extractSourceCode(funcDecl), nil
			}
		}
	}
	return "", nil
}

// Helper to find function or method in a package
func findFunctionOrMethodInPackage(pkgPath, typeName, funcName string) (string, error) {
	files, err := filepath.Glob(filepath.Join(pkgPath, "*.go"))
	if err != nil {
		return "", fmt.Errorf("failed to list files in package: %w", err)
	}

	for _, file := range files {
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, file, nil, parser.AllErrors)
		if err != nil {
			continue // Skip files that fail to parse
		}

		funcSource, err := findFunctionOrMethodInFile(node, typeName, funcName)
		if funcSource != "" || err != nil {
			return funcSource, err
		}
	}
	return "", nil
}

// Extract the source code of a function or method
func extractSourceCode(funcDecl *ast.FuncDecl) string {
	var buf bytes.Buffer
	printer.Fprint(&buf, token.NewFileSet(), funcDecl)
	return buf.String()
}
