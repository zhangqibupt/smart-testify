

# Smart-Testify: AI-Powered Unit Test Generation for Go Code

**Smart-Testify** is a Go-based tool that automatically generates unit test code for Go projects using GitHub Copilot AI. It leverages Copilot's API to generate high-quality unit tests for Go functions and offers flexibility in handling test file generation modes.
    
---   
## Features

- **AI-Powered Test Generation**: Automatically generates unit test code using Copilot AI based on Go function signatures.
- **Supports File and Directory**: Can generate tests for a single Go file or for all Go files within a directory.
- **Flexible Generation Modes**:
  - **`overwrite`**: Overwrites existing test files.
  - **`append`**: Appends new tests to the existing test file.
  - **`skip`**: Skips generating tests for functions that already have tests.
- **Smart Handling of Existing Test Files**: Automatically checks for existing test files to avoid duplicate tests and supports overwriting or appending tests.
- **Flexible Error Handling**: Provides the option to continue execution even after encountering an error with the `ignore-error` flag.

---   
## Installation and Setup

### 1. Install

First, clone the project to your local machine. 
Run `make install` to install the necessary dependencies and build the project. This will create an executable file named `smart-testify` in $GOPATH/bin folder.

```
  
### 2. Set Up Copilot Token

**Smart-Testify** requires a valid Copilot token for generating tests. You need to set up this token for the tool to interact with the Copilot API.

Run the following command to initialize the Copilot token:
```  
smart-testify init-token  
```  
Follow the instructions in the console. This command will store your token in the ~/.copilot_token file, which will be used by the tool during subsequent interactions with Copilot.
### 3 . Run the Program
To generate unit tests for your Go files, run the following command:
```  
smart-testify generate --path /path/to/your/go/file/or/directory --mode overwrite  
```  
#### Available Commands and Flags

-   `init-token`: Initializes the Copilot token. This command should only be run once unless you need to change your token.
-   `generate`: Generates unit test files for the specified Go code.
-   `--path` (`-p`): Specifies the file or directory path for which to generate tests.
-   `--mode` (`-m`): Defines the test generation mode:  `overwrite`,  `append`, or  `skip`.
-   `--ignore-error` (`-c`): If enabled, the tool will continue execution even if an error occurs.

## How It Works

1.  **Initialize Token**: The tool first requires the Copilot token to interact with the Copilot API.
2.  **Read Go Files**: The tool scans the specified files or directories for Go source files.
3.  **Generate Test Code**: Using the Copilot API, the tool generates test code based on the function signatures in the Go code.
4.  **Write Test File**: The generated test code is written to a new  `_test.go` file. If the file already exists, it can be overwritten, appended, or skipped based on the mode selected.
5.  **Code Formatting**: After generating the test code, the tool formats it using  `goimports` to ensure the code adheres to Go's formatting conventions.
## Example

### 1. Generating Test Files

Suppose you have the following Go file  `example.go`:
```  
package example

type Greeter struct {
	Name string
}

func (g *Greeter) Greet() string {
	return "Hello, " + g.Name
}
  
```  
Run the following command to generate a test file for `example.go`:
```  
smart-testify generate -p ./example.go -m overwrite  
```  
The generated test file  `example_test.go` might look like this:
```  
package example

import "testing"

func TestGreeter_Greet(t *testing.T) {
	greeter := &Greeter{Name: "World"}
	got := greeter.Greet()
	want := "Hello, World"
	if got != want {
		t.Errorf("Greet() = %v, want %v", got, want)
	}
}
  
```  
### 2. Generating Tests for Multiple Files

To generate tests for all Go files in a directory, use the directory path:
```  
smart-testify generate -p ./example_dir -m append  
```  
This will generate tests for all Go files in  `example_dir` that don't already have tests.

## Logging

`smart-testify` uses the  `logrus` logging library to log important actions and events. During execution, logs will record key steps, generated test functions, error messages, and other relevant information.

-   **`info` level**: Logs major events and actions.
-   **`debug` level**: Logs detailed information, such as function calls and parameters (enabled by default in development).
-   **`error` level**: Logs error messages.

Logging behavior can be configured via environment variables:

`LOG_LEVEL`: Defines the log level. Possible values are debug, info, warn, and error. Default is info.  
`LOG_FORMAT`: Defines the log format. Set it to json for JSON formatted logs or leave it as the default (text) for human-readable logs.  
The logger is configured to output logs to stdout.
## Frequently Asked Questions (FAQ)

### 1.  **How do I obtain the GitHub Copilot token?**

You can get an API token from  [GitHub Copilot](https://github.com/copilot) and store it in the  `~/.copilot_token` file.

### 2.  **How do I choose a generation mode?**

When running the  `generate` command, use the  `--mode` flag to select one of the following options:

-   `overwrite`: Overwrites an existing test file.
-   `append`: Adds new test functions to an existing test file.
-   `skip`: Skips functions that already have tests.

### 3.  **How can I process multiple Go files in a directory?**

You can specify a directory path using the  `--path` option. The tool will recursively process all Go files in the directory.
  
---  
## Contributing

We welcome contributions to this project. Please follow these steps to submit a pull request:

1. Fork the repository
2. Create a new branch
3. Make your changes
4. Submit a pull request
---  
## License

This project is licensed under the [MIT License](https://opensource.org/licenses/MIT).