package main

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"smart-testify/internal/twinkle"
	"smart-testify/internal/util"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

var (
	modeFlag        string
	filter          string
	ignoreErrorFlag bool
	granularity     string
)

const (
	modeSkip   = "skip"
	modeAppend = "append"

	granularityFile     = "file"
	granularityFunction = "function"
)

// generateCmd generates the Go files or directories
var generateCmd = &cobra.Command{
	Use:   "generate <paths of files or directories>",
	Short: "Generate test files for Go code",
	Args:  cobra.MinimumNArgs(0), // Allow multiple arguments
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			cmd.Help()
			return
		}

		invalidPaths := []string{}
		validPaths := []string{}

		// Step 1: Validate paths and ensure files are Go files
		for _, path := range args {
			fileInfo, err := os.Stat(path)
			if err != nil {
				invalidPaths = append(invalidPaths, path)
				continue
			}

			// If it's a file, check if it has a .go extension
			if !fileInfo.IsDir() && filepath.Ext(path) != ".go" {
				invalidPaths = append(invalidPaths, path+" (not a Go file)")
				continue
			}

			// Add valid paths
			validPaths = append(validPaths, path)
		}

		// Report invalid paths and exit if any
		if len(invalidPaths) > 0 {
			log.Errorf("Invalid paths: %v", invalidPaths)
			return
		}

		log.Infof("Mode: %s", modeFlag)
		log.Infof("Function Filter: %s", filter)
		log.Infof("Ignore Error: %v", ignoreErrorFlag)
		log.Infof("Granularity: %s", granularity)

		// Step 2: Process valid paths
		for _, path := range validPaths {
			fileInfo, _ := os.Stat(path) // No need to check error again, already validated

			log.Infof("Processing Path: %s", path)

			if fileInfo.IsDir() {
				// Process directory
				if err := processDirectory(path); err != nil {
					log.Errorf("Failed to process directory '%s': %v", path, err)
				}
			} else {
				// Process Go file
				if err := processFile(path); err != nil {
					log.Errorf("Failed to process file '%s': %v", path, err)
				}
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
			if filter != "" && granularity == granularityFile {
				// get file name from filePath
				_, fileName := filepath.Split(filePath)

				regex, err := regexp.Compile(filter)
				if err != nil {
					return fmt.Errorf("Failed to compile regex: %v", err)
				}
				if !regex.MatchString(fileName) {
					log.Infof("Skipping file %s due to filter", filePath)
					return nil
				}
			}
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
	var existingTests map[string]*ast.FuncDecl

	if testFileExists {
		if granularity == granularityFile && modeFlag == modeSkip {
			log.Infof("Test file exists for %s, skipping it...", filePath)
			return nil
		}

		log.Infof("Test file exists for %s, loading it...", testFilePath)
		_, existingTests, err = parseTestFile(testFilePath)
		if err != nil {
			return fmt.Errorf("Failed to parse existing test file: %v", err)
		}

	}

	// Initialize final test code which will hold the generated or modified test code
	var generatedTestCode string

	// Process each method and decide if we need to generate or skip test cases
	for _, method := range methods {
		// Generate the test function name by combining the receiver and method name
		testFuncName, err := generateTestFuncName(method)
		if err != nil {
			return fmt.Errorf("Failed to generate test function name: %v", err)
		}

		// Check if a test function already exists for this method
		if _, exists := existingTests[testFuncName]; exists {
			log.Infof("[%s] Test function already exists", testFuncName)

			// If mode is skip, skip generating the test case for this method
			if modeFlag == modeSkip && granularity == granularityFunction {
				log.Infof("[%s] Skipping test generation", testFuncName)
				continue
			}

			// If mode is append, delete the existing test method from the AST
			if modeFlag == modeAppend && granularity == granularityFunction {
				// Remove the old test function from the AST
				// This is achieved by filtering out the existing test function from the declarations
				//modifiedTestCode = removeTestFunction(modifiedTestCode, existingTest)
				log.Infof("[%s] Append more cases", testFuncName)
			}
		}

		log.Infof("[%s] Start to generating test cases", testFuncName)

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
	var originalTestFileCode string
	if testFileExists {
		// just load the existing test file content from testFilePath
		testSourceFile, err := ioutil.ReadFile(testFilePath)
		if err != nil {
			log.Errorf("Failed to read test file %s: %v", testFilePath, err)
			return err
		}
		originalTestFileCode = string(testSourceFile) + "\n"

	} else {
		originalTestFileCode = defaultTestFile(node.Name.Name)
	}

	// Append the generated test code to the existing test file code
	originalTestFileCode += generatedTestCode

	// Write the final generated code to the test file
	if err := writeTestFile(testFilePath, originalTestFileCode); err != nil {
		return fmt.Errorf("Failed to write to test file %s: %v", testFilePath, err)
	}

	if err := util.RunGoImports(testFilePath); err != nil {
		log.Warnf("Failed to run goimports for %s due to %s", testFilePath, err)
	}

	return nil
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

// generateTestFuncName generates the test function name based on receiver type and method name.
func generateTestFuncName(method *ast.FuncDecl) (string, error) {
	// Keep original method name to preserve case
	methodName := method.Name.Name

	if method.Recv != nil && len(method.Recv.List) > 0 {
		// Extract the receiver type name
		pair, err := parseTypeDefination(method.Recv.List[0].Type)
		if err != nil {
			return "", err
		}
		if len(pair) == 0 {
			return "", fmt.Errorf("receiver type not found")
		}

		// Generate test function name: Test_[Receiver]_[Method]
		return "Test_" + pair[0].TypeName + "_" + methodName, nil
	}
	// For regular functions: Test_[Method]
	return "Test_" + methodName, nil
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

		var resp string
		if getGlobalConfig().Model == modelCopilot {
			log.Infof("Using Copilot to generate test cases")
			resp, err = getCopilotClient().Chat(prompt)
			if err != nil {
				return "", fmt.Errorf("Failed to get response from Copilot: %s", err.Error())
			}
		} else {
			log.Infof("Using Twinkle to generate test cases")
			resp, err = twinkle.CallTwinkleAPI(prompt)
			if err != nil {
				return "", fmt.Errorf("Failed to get response from Twinkle: %s", err.Error())
			}
		}

		// Trim the code and add to test code
		log.Infof("Response from AI: %s", resp)
		code, err := extractCode(resp)
		if err != nil {
			return "", fmt.Errorf("Failed to extract code: %s", err.Error())
		}
		testCode += code + "\n\n"
	}
	return testCode, nil
}

// extractCode extracts code from the response between triple backticks.
func extractCode(response string) (string, error) {
	// First, check for the backticks with a language tag (e.g., ```go)
	start := strings.Index(response, "```go")
	if start != -1 {
		// If we find the ` ```go ` tag, adjust the start to skip over the tag
		start += 5
	} else {
		// Otherwise, check for the generic backticks (```).
		start = strings.Index(response, "```")
		if start == -1 {
			return "", errors.New("code not found: missing starting backticks")
		}
		start += 3 // Skip over the backticks
	}

	// Now, find the closing backticks
	end := strings.LastIndex(response, "```")
	if end == -1 || end < start {
		return "", errors.New("code not found: missing ending backticks")
	}

	// Extract the code between the backticks
	code := response[start:end]
	// if multiple \n is found at the start of the code, remove it
	code = strings.TrimLeft(code, "\n")
	code = strings.TrimRight(code, "\n")
	return code, nil
}

type typePair struct {
	PackageName string
	TypeName    string
}

type functionPair struct {
	PackageName string
	TypeName    string
	FuncName    string
}

func uniqueFunctionPair(pairs []functionPair) []functionPair {
	// Create a map to store unique pairs using the combination of importName and Name
	uniqueMap := make(map[string]functionPair)

	for _, pair := range pairs {
		// Create a key by concatenating importName and Name
		key := pair.PackageName + "." + pair.TypeName + "." + pair.FuncName
		// Store the pair in the map (the key ensures uniqueness)
		uniqueMap[key] = pair
	}

	// Convert the map values back to a slice
	var uniquePairs []functionPair
	for _, pair := range uniqueMap {
		uniquePairs = append(uniquePairs, pair)
	}

	return uniquePairs
}

func uniqueTypePair(pairs []typePair) []typePair {
	// Create a map to store unique pairs using the combination of importName and Name
	uniqueMap := make(map[string]typePair)

	for _, pair := range pairs {
		// Create a key by concatenating importName and Name
		key := pair.PackageName + "." + pair.TypeName
		// Store the pair in the map (the key ensures uniqueness)
		uniqueMap[key] = pair
	}

	// Convert the map values back to a slice
	var uniquePairs []typePair
	for _, pair := range uniqueMap {
		uniquePairs = append(uniquePairs, pair)
	}

	return uniquePairs
}

func sortByImportNameAndName(pairs []typePair) {
	sort.Slice(pairs, func(i, j int) bool {
		// First, check if importName is empty
		if pairs[i].PackageName == "" && pairs[j].PackageName != "" {
			return true // Empty importName should come first
		}
		if pairs[i].PackageName != "" && pairs[j].PackageName == "" {
			return false // Non-empty importName should come later
		}
		// If both have the same importName (either both empty or both non-empty), sort by Name
		return pairs[i].TypeName < pairs[j].TypeName
	})
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
	if filter != "" {
		regex, err = regexp.Compile(filter)
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

	customPrompt, err := loadPrompt("")
	if err != nil {
		if err.Error() == "no default prompt configured" {
			return "", fmt.Errorf("no prompt configured - please set a default prompt using: smart-testify config prompt set-default <name>")
		}
		return "", err
	}

	testFuncName, err := generateTestFuncName(method)
	if err != nil {
		return "", fmt.Errorf("failed to generate test function name: %s", err.Error())
	}

	// Generate the final prompt with context
	return fmt.Sprintf(`Generate unit tests for below function: 
%s
%s

The related types and functions definition code is:
%s

You should only output the test function, nothing else. Don't output the package declaration, imports, or any other code.
The test function name should be %s.

%s
`,
		importSectionCode,
		methodCode,
		generatedTypeDefinationCode, testFuncName, customPrompt,
	), nil
}

func generateTypeDefinitionSectionCode(method *ast.FuncDecl, filePath string) (string, error) {
	var allTypePairs []typePair

	// Collect types from receiver, parameters, and results
	if method.Recv != nil {
		// Gather source code for the receiver type
		pairs, err := parseTypeDefination(method.Recv.List[0].Type)
		if err != nil {
			return "", err
		}
		if len(pairs) == 0 {
			return "", fmt.Errorf("receiver type not found")
		}
		allTypePairs = append(allTypePairs, pairs...)
	}

	if method.Type.Params != nil {
		for _, param := range method.Type.Params.List {
			pairs, err := parseTypeDefination(param.Type)
			if err != nil {
				return "", err
			}
			allTypePairs = append(allTypePairs, pairs...)
		}
	}

	if method.Type.Results != nil {
		for _, result := range method.Type.Results.List {
			pairs, err := parseTypeDefination(result.Type)
			if err != nil {
				return "", err
			}
			allTypePairs = append(allTypePairs, pairs...)
		}
	}

	// Gather types and functions used in the method body
	usedFunctions, usedTypes, err := collectTypesAndFunctionsFromBody(method.Body)
	if err != nil {
		return "", err
	}

	// Append these used types and functions to the type list
	allTypePairs = append(allTypePairs, usedTypes...)

	// Generate type-related code
	generatedTypeDefinationCode, err := generateTypeDefinition(filePath, allTypePairs)
	if err != nil {
		return "", err
	}

	// Optionally, add function definitions found in the method body (e.g., via FindFunctionSource)
	sortFunctionPairs(usedFunctions)
	for _, funcDef := range usedFunctions {
		funcSource, err := util.FindFunctionSource(filePath, funcDef.PackageName, funcDef.TypeName, funcDef.FuncName)
		if err != nil {
			return "", nil
		}
		if len(strings.TrimSpace(funcSource)) > 0 {
			funcSource = fmt.Sprintf("Package: %s \nMethod: %s\n", funcDef.PackageName, funcDef.FuncName) + funcSource
			generatedTypeDefinationCode += "\n\n" + funcSource
		}
	}

	return generatedTypeDefinationCode, nil
}

func sortFunctionPairs(functions []functionPair) {
	sort.Slice(functions, func(i, j int) bool {
		// Compare PackageName, with empty ones coming first
		if functions[i].PackageName != functions[j].PackageName {
			return functions[i].PackageName < functions[j].PackageName
		}
		// Compare TypeName
		if functions[i].TypeName != functions[j].TypeName {
			return functions[i].TypeName < functions[j].TypeName
		}
		// Compare FuncName
		return functions[i].FuncName < functions[j].FuncName
	})
}

// collectTypesAndFunctionsFromBody extracts all types and function names used within the method body.
func collectTypesAndFunctionsFromBody(body *ast.BlockStmt) ([]functionPair, []typePair, error) {
	if body == nil {
		return nil, nil, nil
	}

	var usedFunctions []functionPair
	var usedTypes []typePair

	ast.Inspect(body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.CallExpr:
			if sel, ok := x.Fun.(*ast.SelectorExpr); ok {
				if pkgIdent, ok := sel.X.(*ast.Ident); ok {
					if pkgIdent.Obj != nil && pkgIdent.Obj.Kind == ast.Var {
						// ignore this case for now (e.g., c.Method())
						//usedFunctions = append(usedFunctions, functionPair{
						//	PackageName: "",
						//	TypeName:    pkgIdent.Name, // 存入变量名作为类型
						//	FuncName:    sel.Sel.Name,
						//})
					} else {
						// 直接使用 Ident 名称作为 package 名，不依赖 pkgIdent.Obj
						usedFunctions = append(usedFunctions, functionPair{
							PackageName: pkgIdent.Name,
							TypeName:    "",
							FuncName:    sel.Sel.Name,
						})
					}
				} else {
					// 可能是对象方法调用 (var.Method())，尝试解析 var 的类型
					//var typeName string
					//if typ, ok := sel.X.(*ast.Ident); ok {
					//	typeName = typ.Name // 直接获取变量的类型名称
					//}
					//usedFunctions = append(usedFunctions, functionPair{
					//	PackageName: "",
					//	TypeName:    typeName,
					//	FuncName:    sel.Sel.Name,
					//})
				}
			} else if funIdent, ok := x.Fun.(*ast.Ident); ok {
				// 直接函数调用 (Func())
				usedFunctions = append(usedFunctions, functionPair{
					PackageName: "",
					TypeName:    "",
					FuncName:    funIdent.Name,
				})
			}
		case *ast.Ident:
			if x.Obj != nil && x.Obj.Kind == ast.Typ {
				usedTypes = append(usedTypes, typePair{
					PackageName: "",
					TypeName:    x.Name,
				})
			}
		case *ast.SelectorExpr:
			if pkgIdent, ok := x.X.(*ast.Ident); ok {
				// 修复：判断是否是包选择符，而不是类型
				if pkgIdent.Obj == nil || pkgIdent.Obj.Kind != ast.Typ {
					usedTypes = append(usedTypes, typePair{
						PackageName: pkgIdent.Name,
						TypeName:    x.Sel.Name,
					})
				}
			}
		}
		return true
	})

	return uniqueFunctionPair(usedFunctions), uniqueTypePair(usedTypes), nil
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
	uniquePairs := uniqueTypePair(pairs)
	sortByImportNameAndName(uniquePairs)

	var resultCode strings.Builder

	for _, pair := range uniquePairs {
		sourceCode, err := util.FindTypeSource(filePath, pair.PackageName, pair.TypeName)
		if err != nil {
			return "", err
		}
		if sourceCode != "" {
			if pair.PackageName == "" {
				resultCode.WriteString(fmt.Sprintf("Model: %s\nDefinition:\n%s\n", pair.TypeName, sourceCode))
			} else {
				resultCode.WriteString(fmt.Sprintf("Package: %s \nModel: %s\nDefinition:\n%s\n", pair.PackageName, pair.TypeName, sourceCode))
			}
		}
	}

	return resultCode.String(), nil
}

func init() {
	generateCmd.Flags().StringVarP(&modeFlag, "mode", "m", modeAppend, "Mode controls whether the test cases will be generated when the test function/file(depends on the --granularity flag) already exists. Possible values: skip, append.")
	generateCmd.Flags().StringVarP(&filter, "filter", "f", "", "Regex filter for functions/filter to generate tests for")
	generateCmd.Flags().StringVarP(&granularity, "granularity", "g", granularityFunction, "Used with the append mode: file or function. "+
		"When mode=skip and granularity=file, the entire test file is skipped. "+
		"When mode=skip and granularity=function, the test function is skipped. "+
		"When mode=append, no matter the granularity, the test function is appended to the test file.")
	generateCmd.Flags().BoolVarP(&ignoreErrorFlag, "ignore-error", "c", false, "When Smart-Testify is processing multiple fils, it will stop processing when it encounters an error. However, you can use --ignore-error to ignore the error and continue processing the next file.")
}
