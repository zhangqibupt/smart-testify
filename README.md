
# Smart-Testify

Smart-Testify is a tool that generates unit test files for Go code using AI. It supports various commands for configuring settings, managing prompts, and generating tests. Below is a concise overview of the commands and usage.

## Table of Contents

- [Installation](#installation)
- [Usage](#usage)
- [Commands](#commands)
- [Flags](#flags)
- [Examples](#examples)
- [Contributing](#contributing)
- [License](#license)

## Installation

Smart-Testify can be installed by downloading the latest release or building from source.

### To build from source:

Clone the repository, go into the root folder and run
```bash  
make install
```  

## Usage



### Commands

#### `config`
Configure settings.

- **`use`**: Set the AI model to use (`copilot` or `twinkle`).
- **`show`**: Display current configuration.
- **`prompt`**: Manage the default prompt for test generation.
  - **`edit`**: Edit the default prompt.
  - **`reset`**: Reset the prompt to its original state.
  - **`show`**: Show the current prompt.

#### `generate`
Generate unit test files for Go code.

- **`generate <path>`**: Generate tests for a specified Go file or directory.
  - **`--mode`** (`-m`): Mode for test generation (`append` or `skip`). Defaults to `append`.
  - **`--filter`** (`-f`): Regex filter for functions to generate tests for. Wildcard is supported, but you need to wrap it in quotes. For example `-f "Test*"`.
  - **`--granularity`** (`-g`): Granularity of test generation (`file` or `function`).
  - **`--ignore-error`** (`-c`): Continue processing if an error occurs. Defaults to `false`.

## Examples

1. **Set the AI model to Twinkle**:
```bash  
 smart-testify config use twinkle  
```  

2. **Generate tests for a Go file**:
```bash  
 smart-testify generate /path/to/your/file.go  
```  

3. **Edit the prompt for test case generation**:
```bash  
 smart-testify config prompt edit  
```  

4. **Reset the prompt to the default**:
```bash  
 smart-testify config prompt reset  
```  


## Contributing

Contributions are welcome! If you find any bugs or want to add new features, please fork the repository, make changes, and create a pull request. Before submitting your pull request, ensure that you have run all tests and followed the coding style guidelines.

## License

Smart-Testify is licensed under the MIT License. See [LICENSE](LICENSE) for more details.