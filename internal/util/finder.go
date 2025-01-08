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
	// 正则表达式用于从路径中提取包名部分
	// 如果路径有版本号，需要去掉版本号, 如 github.com/volatiletech/null/v9
	re := regexp.MustCompile(`/v\d+$`)

	for _, imp := range imports {
		// 获取实际的导入路径，去掉引号
		importPath := strings.Trim(imp.Path.Value, `"`)

		// 如果导入使用了别名
		if imp.Name != nil {
			// 检查别名是否与 pkgShortName 匹配
			if imp.Name.Name == pkgShortName {
				return importPath, true
			}
		} else {
			// 如果没有别名，提取包名（通过正则表达式）
			importPath = re.ReplaceAllString(importPath, "")
			parts := strings.Split(importPath, "/")
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

	// 添加 type 关键字和类型名称
	buf.WriteString("type ")

	// 格式化 TypeSpec 的内容
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

		// Skip directories and non-Go files
		if info.IsDir() || filepath.Ext(info.Name()) != ".go" {
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

func collectImports(node *ast.File) []string {
	var imports []string
	for _, imp := range node.Imports {
		importPath := strings.Trim(imp.Path.Value, `"`)
		imports = append(imports, importPath)
	}
	return imports
}

func formatNode(buf *strings.Builder, node ast.Node) error {
	return printer.Fprint(buf, token.NewFileSet(), node)
}
