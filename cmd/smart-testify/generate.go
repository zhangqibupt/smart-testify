package main

import (
	"bytes"
	"fmt"
	"github.com/spf13/cobra"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"smart-testify/internal/util"
	"strings"
)

// generateCmd generates the Go files or directories
var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate test files for Go code",
	Run: func(cmd *cobra.Command, args []string) {
		if pathFlag == "" {
			log.Errorf("Path must be specified")
			return
		}

		// Ensure the token is valid
		if err := client.LoadToken(); err != nil {
			log.Errorf("Error loading token: %v", err)
			return
		}

		// Process file or directory
		fileInfo, err := os.Stat(pathFlag)
		if err != nil {
			log.Errorf("Failed to get file info: %v, please check the path", err)
			return
		}

		if fileInfo.IsDir() {
			if err := processDirectory(pathFlag); err != nil {
				log.Errorf("Failed to process directory: %v", err)
			}
		} else {
			if err := processFile(pathFlag); err != nil {
				log.Errorf("Failed to process file: %v", err)
			}
		}
	},
}

func processDirectory(path string) error {
	return filepath.Walk(path, func(filePath string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(filePath, ".go") && !strings.HasSuffix(filePath, "_test.go") {
			if err := processFile(filePath); err != nil {
				log.Errorf("Failed to process file: %v", err)
				if !ignoreErrorFlag {
					return err
				}
			}
		}
		return nil
	})
}

func processFile(filePath string) error {
	log.Infof("Starting to process file: %s", filePath)
	defer log.Infof("Finished processing file: %s", filePath)

	sourceFileSet, node, err := parseGoFile(filePath)
	if err != nil {
		return fmt.Errorf("Failed to parse file %s: %v", filePath, err)
	}
	// Collect methods and types
	methods, err := collectMethods(node)
	if err != nil {
		return fmt.Errorf("Failed to collect methods and types: %v", err)
	}
	if len(methods) == 0 {
		log.Infof("No methods found in file %s, skipping it...", filePath)
		return nil
	}

	// Test file path
	testFilePath := strings.TrimSuffix(filePath, ".go") + "_test.go"
	testFileExists := fileExists(testFilePath)
	var existingNode *ast.File
	var existingTests map[string]*ast.FuncDecl

	if testFileExists {
		if granularity == "file" && modeFlag == "skip" {
			log.Infof("Test file exists for %s, skipping it...", filePath)
			return nil
		}

		if granularity == "file" && modeFlag == "overwrite" {
			testFileExists = false
		} else {
			existingNode, existingTests, err = parseTestFile(testFilePath)
			if err != nil {
				return fmt.Errorf("Failed to parse existing test file: %v", err)
			}
		}

	}

	// Initialize final test code which will hold the generated or modified test code
	var generatedTestCode string
	var modifiedTestCode []ast.Decl

	// Process each method and decide if we need to generate or skip test cases
	for _, method := range methods {
		// Generate the test function name by combining the receiver and method name
		testFuncName, err := generateTestFuncName(method)
		if err != nil {
			return fmt.Errorf("Failed to generate test function name: %v", err)
		}

		// Check if a test function already exists for this method
		if existingTest, exists := existingTests[testFuncName]; exists {
			log.Infof("[%s] Test function already exists", testFuncName)

			// If mode is skip, skip generating the test case for this method
			if modeFlag == "skip" && granularity == "function" {
				log.Infof("[%s] Skipping test generation", testFuncName)
				modifiedTestCode = append(modifiedTestCode, existingTest)
				continue
			}

			// If mode is append, generate a new test function with a unique name
			if modeFlag == "append" && granularity == "function" {
				// Append "_old" to the test function name to avoid conflicts
				newTestFuncName := testFuncName + "_old"
				// Create the new function declaration with the "_old" name
				newTestDecl := *existingTest
				newTestDecl.Name.Name = newTestFuncName
				newTestDecl.Doc = existingTest.Doc

				// Add the modified function to the list of declarations
				modifiedTestCode = append(modifiedTestCode, &newTestDecl)
				log.Infof("[%s] Rename existing test function to %s", testFuncName, newTestFuncName)
			}

			// If mode is overwrite, delete the existing test method from the AST
			if modeFlag == "overwrite" && granularity == "function" {
				// Remove the old test function from the AST
				// This is achieved by filtering out the existing test function from the declarations
				//modifiedTestCode = removeTestFunction(modifiedTestCode, existingTest)
				log.Infof("[%s] Overwritting test function", testFuncName)
			}
		}

		log.Infof("[%s] Generating test cases using Copilot......", testFuncName)

		testMethodSourceCode, err := generateTestCases(sourceFileSet, []*ast.FuncDecl{method}, filePath)
		if err != nil {
			return fmt.Errorf("Failed to generate test cases for method %s: %v", method.Name.Name, err)
		}
		generatedTestCode += testMethodSourceCode
	}

	if generatedTestCode == "" {
		log.Infof("No test cases generated for file %s", filePath)
		return nil
	}

	// Generate the modified test file content by modifying the AST
	var modifiedTestFileCode string
	if testFileExists {
		modifiedTestFileCode, err = generateTestFileFromAST(sourceFileSet, existingNode, modifiedTestCode)
		if err != nil {
			return fmt.Errorf("Failed to generate test file from AST: %v", err)
		}
	} else {
		modifiedTestFileCode = defaultTestFile(node.Name.Name)
	}

	// Append the generated test code to the existing test file code
	modifiedTestFileCode += generatedTestCode

	// TODO generate the new test file
	// Write the final generated code to the test file
	if err := writeTestFile(testFilePath, modifiedTestFileCode); err != nil {
		return fmt.Errorf("Failed to write to test file %s: %v", testFilePath, err)
	}

	if err := util.RunGoImports(testFilePath); err != nil {
		log.Warnf("Failed to run goimports for %s due to %s", testFilePath, err)
	}

	return err
}

func defaultTestFile(packageName string) string {
	return fmt.Sprintf(`
// Code generated by AI.
package %s

import (
	"testing"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
	"github.com/volatiletech/null/v9"
)

`, packageName)
}

// writeTestFile writes the modified test code to the test file
func writeTestFile(testFilePath, finalCode string) error {
	// If the file already exists, overwrite it
	err := ioutil.WriteFile(testFilePath, []byte(finalCode), 0644)
	if err != nil {
		log.Errorf("Failed to write to file %s: %v", testFilePath, err)
		return err
	}
	return nil
}

// parseTestFile parses the existing test file and returns a map of existing test function names
func parseTestFile(filePath string) (*ast.File, map[string]*ast.FuncDecl, error) {
	existingTests := make(map[string]*ast.FuncDecl)

	node, err := parser.ParseFile(token.NewFileSet(), filePath, nil, parser.AllErrors)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to parse test file: %v", err)
	}

	ast.Inspect(node, func(n ast.Node) bool {
		if funcDecl, ok := n.(*ast.FuncDecl); ok {
			if strings.HasPrefix(funcDecl.Name.Name, "Test") {
				existingTests[funcDecl.Name.Name] = funcDecl
			}
		}
		return true
	})
	return node, existingTests, nil
}

// generateTestFileFromAST generates the final test file code from the modified AST.
func generateTestFileFromAST(fset *token.FileSet, existingNode *ast.File, modifiedTestCode []ast.Decl) (string, error) {
	var buf bytes.Buffer
	node := &ast.File{
		Decls: modifiedTestCode,
		Name:  existingNode.Name,
	}

	// Format the AST to source code
	err := format.Node(&buf, fset, node)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

// generateTestFuncName generates the test function name based on receiver type and method name.
func generateTestFuncName(method *ast.FuncDecl) (string, error) {
	// Assuming method has a receiver
	if method.Recv != nil && len(method.Recv.List) > 0 {
		// Extract the receiver type name
		pair, err := parseTypeDefination(method.Recv.List[0].Type)
		if err != nil {
			return "", err
		}
		if len(pair) == 0 {
			return "", fmt.Errorf("receiver type not found")
		}

		// Generate test function name: Test[Receiver][Method]
		return "Test" + pair[0].typeName + "_" + method.Name.Name, nil
	}
	return "Test" + method.Name.Name, nil
}

// Check if a file exists
func fileExists(filePath string) bool {
	_, err := os.Stat(filePath)
	return !os.IsNotExist(err)
}

// Parse a Go file and return the AST node
func parseGoFile(filePath string) (*token.FileSet, *ast.File, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.AllErrors)
	return fset, node, err
}

// Generate test cases for each method
func generateTestCases(fset *token.FileSet, methods []*ast.FuncDecl, filePath string) (string, error) {
	var testCode string
	for _, method := range methods {
		prompt, err := generatePrompt(fset, method, filePath)
		if err != nil {
			return "", fmt.Errorf("Failed to generate prompt: %s", err.Error())
		}

		log.Infof("Prompt for method %s: %s", method.Name.Name, prompt)

		// Get the response from the Copilot client
		response, err := client.Chat(prompt)
		if err != nil {
			return "", fmt.Errorf("Failed to get response from Copilot: %s", err.Error())
		}

		// Trim the code and add to test code
		testCode += trimCode(response) + "\n\n"
	}
	return testCode, nil
}

// trimCode removes ```go at the beginning and ``` at the end of the code block, if present
func trimCode(response string) string {
	// Trim leading and trailing whitespace
	response = strings.TrimSpace(response)

	// Check if the response starts with "```go"
	if strings.HasPrefix(response, "```go") {
		// Remove the "```go" at the start
		response = strings.TrimPrefix(response, "```go")
	}

	// Check if the response ends with "```"
	if strings.HasSuffix(response, "```") {
		// Remove the "```" at the end
		response = strings.TrimSuffix(response, "```")
	}

	// Trim any leading or trailing whitespace again after trimming the code block markers
	return strings.TrimSpace(response)
}

type typePair struct {
	importName string
	typeName   string
}

func parseTypeDefination(expr ast.Expr) ([]typePair, error) {
	switch t := expr.(type) {
	case *ast.Ident:
		return []typePair{
			{
				"",
				t.Name,
			},
		}, nil
	case *ast.StarExpr:
		return parseTypeDefination(t.X)
	case *ast.ArrayType:
		return parseTypeDefination(t.Elt)
	case *ast.MapType:
		keyPairs, err := parseTypeDefination(t.Key)
		if err != nil {
			return []typePair{}, err
		}

		valuePairs, err := parseTypeDefination(t.Value)
		if err != nil {
			return []typePair{}, err
		}
		return append(keyPairs, valuePairs...), nil
	case *ast.SelectorExpr:
		if pkgIdent, ok := t.X.(*ast.Ident); ok {
			return []typePair{
				{
					pkgIdent.Name,
					t.Sel.Name,
				},
			}, nil
		}
		return []typePair{}, fmt.Errorf("unsupported selector expression: %T", t.X)
	case *ast.InterfaceType:
		return []typePair{}, nil
	default:
		log.Warnf("Unsupported type: %T", t)
		return []typePair{}, nil
	}
}

func collectMethods(node *ast.File) ([]*ast.FuncDecl, error) {
	var methods []*ast.FuncDecl

	var regex *regexp.Regexp
	var err error
	if functionFilter != "" {
		regex, err = regexp.Compile(functionFilter)
		if err != nil {
			return nil, fmt.Errorf("Failed to compile regex: %v", err)
		}
	}

	ast.Inspect(node, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncDecl:
			if regex == nil {
				methods = append(methods, x)
			} else if regex != nil && regex.MatchString(x.Name.Name) {
				methods = append(methods, x)
			}
		}
		return true
	})
	return methods, nil
}

func generatePrompt(fset *token.FileSet, method *ast.FuncDecl, filePath string) (string, error) {
	// generate imports
	importSectionCode, err := generateImportSectionCode(filePath)
	if err != nil {
		log.Errorf("Failed to generate import section code: %v", err)
		return "", err
	}

	var methodCode string
	var methodBuf bytes.Buffer
	if err := format.Node(&methodBuf, fset, method); err != nil {
		return "", fmt.Errorf("failed to generate source code for %s due to %s", method.Name.Name, err.Error())
	}
	methodCode += methodBuf.String() + "\n"

	generatedTypeDefinationCode, err := generateTypeDefinitionSectionCode(method, filePath)
	if err != nil {
		log.Errorf("Failed to generate type definition section code: %v", err)
		return "", err
	}

	customPrompt, err := loadPrompt()
	if err != nil {
		return "", err
	}

	if customPrompt == "" {
		log.Warnf("No custom prompt found, using default prompt")
		customPrompt = defaultPrompt
	}

	// Generate the final prompt with context
	return fmt.Sprintf(`Generate unit tests for below function: 
%s
%s

The related type definition code is:
%s

You should only output the code in one function, nothing else.

%s
`,
		importSectionCode,
		methodCode,
		generatedTypeDefinationCode, customPrompt,
	), nil
}

func generateTypeDefinitionSectionCode(method *ast.FuncDecl, filePath string) (string, error) {
	var allPairs []typePair
	if method.Recv != nil {
		// Gather source code for the receiver, params, and returns types
		pairs, err := parseTypeDefination(method.Recv.List[0].Type)
		if err != nil {
			return "", err
		}
		if len(pairs) == 0 {
			return "", fmt.Errorf("receiver type not found")
		}
		allPairs = append(allPairs, pairs...)
	}

	if method.Type.Params != nil {
		for _, param := range method.Type.Params.List {
			pairs, err := parseTypeDefination(param.Type)
			if err != nil {
				return "", err
			}
			allPairs = append(allPairs, pairs...)
		}
	}

	if method.Type.Results != nil {
		for _, result := range method.Type.Results.List {
			pairs, err := parseTypeDefination(result.Type)
			if err != nil {
				return "", err
			}
			allPairs = append(allPairs, pairs...)
		}
	}

	// genereate type related code
	generatedTypeDefinationCode, err := generateTypeDefinition(filePath, allPairs)
	if err != nil {
		return "", err
	}
	return generatedTypeDefinationCode, nil
}

func generateImportSectionCode(path string) (string, error) {
	// Open the Go file
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Parse the Go file
	fs := token.NewFileSet()
	node, err := parser.ParseFile(fs, path, file, parser.ImportsOnly)
	if err != nil {
		return "", fmt.Errorf("failed to parse file: %w", err)
	}

	// Collect the imports
	var imports []string
	for _, imp := range node.Imports {
		// Clean the import string and store it
		imports = append(imports, imp.Path.Value)
	}

	// Format the imports into a Go code string
	var importCode strings.Builder
	importCode.WriteString("import (\n")
	for _, imp := range imports {
		importCode.WriteString(fmt.Sprintf("\t%v\n", imp))
	}
	importCode.WriteString(")\n")

	return importCode.String(), nil
}

func generateTypeDefinition(filePath string, pairs []typePair) (string, error) {
	if len(pairs) == 0 {
		return "", nil
	}

	// Use a map to ensure unique pairs based on importName and typeName
	uniquePairs := make(map[string]typePair)
	for _, pair := range pairs {
		// Create a unique key by combining importName and typeName
		key := fmt.Sprintf("%s:%s", pair.importName, pair.typeName)

		// Only add to map if the key doesn't already exist
		if _, exists := uniquePairs[key]; !exists {
			uniquePairs[key] = pair
		}
	}

	var resultCode strings.Builder

	// Process pairs with empty importName first
	for _, pair := range uniquePairs {
		if pair.importName == "" {
			sourceCode, err := util.FindTypeSource(filePath, pair.importName, pair.typeName)
			if err != nil {
				return "", err
			}
			if sourceCode != "" {
				resultCode.WriteString(sourceCode + "\n")
			}
		}
	}

	// Process pairs with non-empty importName
	for _, pair := range uniquePairs {
		if pair.importName != "" {
			sourceCode, err := util.FindTypeSource(filePath, pair.importName, pair.typeName)
			if err != nil {
				return "", err
			}
			if sourceCode != "" {
				resultCode.WriteString(fmt.Sprintf("Package: %s\nModel: %s\nDefinition:\n%s\n", pair.importName, pair.typeName, sourceCode))
			}
		}
	}

	return resultCode.String(), nil
}

func init() {
	generateCmd.Flags().StringVarP(&pathFlag, "path", "p", "", "Path to the file or directory to generate tests for")
	generateCmd.Flags().StringVarP(&modeFlag, "mode", "m", "overwrite", "Mode for test file generation: overwrite, append, or skip")
	generateCmd.Flags().StringVarP(&functionFilter, "filter", "f", "", "Regex filter for functions to generate tests for")
	generateCmd.Flags().StringVarP(&granularity, "granularity", "g", "file", "Used with the append mode to specify the granularity of test generation: file or function. "+
		"When mode=overwrite and granularity=file, the entire test file is overwritten. "+
		"When mode=skip and granularity=file, the entire test file is skipped. "+
		"When mode=overwrite and granularity=function, the test function is overwritten. "+
		"When mode=skip and granularity=function, the test function is skipped. "+
		"When mode=append, no matter the granularity, the test function is appended to the test file.")
}
