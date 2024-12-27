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
	"smart-testify/internal/util"
	"strings"
)

// generate generates the Go files or directories
var generate = &cobra.Command{
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
	fset, node, err := parseGoFile(filePath)
	if err != nil {
		return fmt.Errorf("Failed to parse file %s: %v", filePath, err)
	}

	// Collect methods and types
	methods, types := collectMethodsAndTypes(node)
	if len(methods) == 0 {
		log.Infof("No methods found in file %s, skipping it...", filePath)
		return nil
	}

	// Test file path
	testFilePath := strings.TrimSuffix(filePath, ".go") + "_test.go"

	// Check if the test file exists
	testFileExists := fileExists(testFilePath)
	var existingTestCode []byte
	if testFileExists {
		// Read the existing test file if it exists
		existingTestCode, err = ioutil.ReadFile(testFilePath)
		if err != nil {
			return fmt.Errorf("Failed to read existing test file %s: %v", testFilePath, err)
		}
	}

	// Parse existing test file to find existing test functions
	existingNode, existingTests, err := parseTestFile(fset, string(existingTestCode))
	if err != nil {
		return fmt.Errorf("Failed to parse existing test file: %v", err)
	}

	// Initialize final test code which will hold the generated or modified test code
	var generatedTestCode string
	var modifiedTestCode []ast.Decl

	// Process each method and decide if we need to generate or skip test cases
	for _, method := range methods {
		// Generate the test function name by combining the receiver and method name
		testFuncName := generateTestFuncName(method)

		// Check if a test function already exists for this method
		if existingTest, exists := existingTests[testFuncName]; exists {
			log.Infof("[%s] Test function already exists", testFuncName)

			// If mode is skip, skip generating the test case for this method
			if modeFlag == "skip" {
				log.Infof("[%s] Skipping test generation", testFuncName)
				modifiedTestCode = append(modifiedTestCode, existingTest)
				continue
			}

			// If mode is append, generate a new test function with a unique name
			if modeFlag == "append" {
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
			if modeFlag == "overwrite" {
				// Remove the old test function from the AST
				// This is achieved by filtering out the existing test function from the declarations
				//modifiedTestCode = removeTestFunction(modifiedTestCode, existingTest)
				log.Infof("[%s] Overwritting test function", testFuncName)
			}
		}

		log.Infof("[%s] Generating test cases using Copilot", testFuncName)

		testMethodSourceCode, err := generateTestCases(fset, []*ast.FuncDecl{method}, types, filePath)
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
	modifiedTestFileCode, err := generateTestFileFromAST(fset, existingNode, modifiedTestCode)
	if err != nil {
		return fmt.Errorf("Failed to generate test file from AST: %v", err)
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
func parseTestFile(fset *token.FileSet, existingTestCode string) (*ast.File, map[string]*ast.FuncDecl, error) {
	existingTests := make(map[string]*ast.FuncDecl)
	node, err := parser.ParseFile(fset, "", existingTestCode, parser.AllErrors)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to parse test file: %v", err)
	}

	// Traverse AST and collect test functions
	ast.Inspect(node, func(n ast.Node) bool {
		if funcDecl, ok := n.(*ast.FuncDecl); ok {
			// Add only Test functions
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
func generateTestFuncName(method *ast.FuncDecl) string {
	// Assuming method has a receiver
	if len(method.Recv.List) > 0 {
		// Extract the receiver type name
		receiverType := getTypeString(method.Recv.List[0].Type)
		// Generate test function name: Test[Receiver][Method]
		return "Test" + receiverType + "_" + method.Name.Name
	}
	return "Test" + method.Name.Name
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
func generateTestCases(fset *token.FileSet, methods []*ast.FuncDecl, types map[string]*ast.TypeSpec, filePath string) (string, error) {
	var testCode string
	for _, method := range methods {
		prompt, err := generatePrompt(fset, method, types, filePath)
		if err != nil {
			return "", fmt.Errorf("Failed to generate prompt: %s", err.Error())
		}

		log.Debugf("Prompt for method %s: %s", method.Name.Name, prompt)

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

func getTypeString(expr ast.Expr) string {
	// 递归处理指针类型，直到找到基础类型
	switch t := expr.(type) {
	case *ast.Ident:
		// 基本类型（例如 int, string）
		return t.Name
	case *ast.StarExpr:
		// 指针类型，递归获取指针指向的类型
		return getTypeString(t.X)
	case *ast.ArrayType:
		// 数组类型，递归获取元素类型并忽略数组
		return getTypeString(t.Elt)
	case *ast.MapType:
		// 映射类型（例如 map[string]int）
		return fmt.Sprintf("map[%v]%v", getTypeString(t.Key), getTypeString(t.Value))
	default:
		// 返回类型的完整信息
		return fmt.Sprintf("%T", expr)
	}
}

func collectMethodsAndTypes(node *ast.File) ([]*ast.FuncDecl, map[string]*ast.TypeSpec) {
	types := make(map[string]*ast.TypeSpec)
	var methods []*ast.FuncDecl

	// 遍历声明的类型和函数
	ast.Inspect(node, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.TypeSpec:
			if _, ok := x.Type.(*ast.StructType); ok {
				types[x.Name.Name] = x
			}
		case *ast.FuncDecl:
			methods = append(methods, x)
		}
		return true
	})
	return methods, types
}

func getTypeSourceCode(typeName string, fset *token.FileSet, types map[string]*ast.TypeSpec) (string, error) {
	t, ok := types[typeName]
	if !ok {
		return "", nil
	}
	var structBuf bytes.Buffer // Buffer implements io.Writer
	if err := format.Node(&structBuf, fset, t); err != nil {
		return "", fmt.Errorf("failed to generate source code for type %s due to %s", typeName, err)
	}
	return structBuf.String(), nil
}

func generatePrompt(fset *token.FileSet, method *ast.FuncDecl, types map[string]*ast.TypeSpec, filePath string) (string, error) {
	var methodBuf bytes.Buffer
	err := format.Node(&methodBuf, fset, method)
	if err != nil {
		return "", fmt.Errorf("failed to generate source code for %s due to %s", method.Name.Name, err.Error())
	}

	// Gather source code for the receiver, params, and returns types
	receiverTypeName := getTypeString(method.Recv.List[0].Type)
	receiverCode, err := getTypeSourceCode(receiverTypeName, fset, types) // Receiver type
	if err != nil {
		return "", err
	}
	paramTypesCode := ""
	for _, param := range method.Type.Params.List {
		paramTypeName := getTypeString(param.Type)
		sourceCode, err := getTypeSourceCode(paramTypeName, fset, types)
		if err != nil {
			return "", err
		}
		paramTypesCode += sourceCode + "\n"
	}
	returnTypesCode := ""
	for _, result := range method.Type.Results.List {
		resultTypeName := getTypeString(result.Type)
		sourceCode, err := getTypeSourceCode(resultTypeName, fset, types)
		if err != nil {
			return "", err
		}
		returnTypesCode += sourceCode + "\n"
	}

	// Generate the final prompt with context
	return fmt.Sprintf(`Generate unit tests for this function. : 
%s.
%s 
%s 
%s 
The output must meet below conditions. 
1. Only output the code in one function, nothing else. 
2. Should include success and failure cases, and include edge cases. When DB operation is involved, you should include db error. Make your best to cover 100 percent of the code. 
3. When it involve gorm DB operations, you should start sqlite in memory to mock it. For example gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{SingularTable: true},
	})
4. When function has receiver, you should include it in the test name. For example, TestLuContentRatingDao_GetALL for function GetALL in LuContentRatingDao.
5. For each function you generated, you should include a comment to declare this function is generated by AI.
`,
		methodBuf.String(),
		receiverCode,
		paramTypesCode,
		returnTypesCode,
	), nil
}
