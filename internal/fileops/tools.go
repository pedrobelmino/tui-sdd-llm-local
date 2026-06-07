// Package fileops provides file-system tools that the Ollama model can invoke
// during the implement/run workflow phases.
//
// Each tool follows the Ollama function-calling schema and is executed by
// Execute, which is passed as a ToolExecutor to ollama.ChatWithTools.
package fileops

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pedrobelmino/tui-sdd-llm-local/internal/ollama"
)

// Definitions returns the full list of file-operation tool definitions.
func Definitions() []ollama.ToolDef {
	return []ollama.ToolDef{
		{
			Type: "function",
			Function: ollama.ToolFunction{
				Name:        "write_file",
				Description: "Create or overwrite a file with the given content. Creates parent directories automatically.",
				Parameters: ollama.ToolParameters{
					Type: "object",
					Properties: map[string]ollama.ToolPropertySchema{
						"path":    {Type: "string", Description: "File path relative to the project root"},
						"content": {Type: "string", Description: "Complete file content to write"},
					},
					Required: []string{"path", "content"},
				},
			},
		},
		{
			Type: "function",
			Function: ollama.ToolFunction{
				Name:        "read_file",
				Description: "Read an existing file and return its content.",
				Parameters: ollama.ToolParameters{
					Type: "object",
					Properties: map[string]ollama.ToolPropertySchema{
						"path": {Type: "string", Description: "File path relative to the project root"},
					},
					Required: []string{"path"},
				},
			},
		},
		{
			Type: "function",
			Function: ollama.ToolFunction{
				Name:        "edit_file",
				Description: "Replace an exact string in a file. Fails if old_content is not found exactly once.",
				Parameters: ollama.ToolParameters{
					Type: "object",
					Properties: map[string]ollama.ToolPropertySchema{
						"path":        {Type: "string", Description: "File path relative to the project root"},
						"old_content": {Type: "string", Description: "The exact substring to find and replace"},
						"new_content": {Type: "string", Description: "The replacement content"},
					},
					Required: []string{"path", "old_content", "new_content"},
				},
			},
		},
		{
			Type: "function",
			Function: ollama.ToolFunction{
				Name:        "delete_file",
				Description: "Delete a file from disk.",
				Parameters: ollama.ToolParameters{
					Type: "object",
					Properties: map[string]ollama.ToolPropertySchema{
						"path": {Type: "string", Description: "File path relative to the project root"},
					},
					Required: []string{"path"},
				},
			},
		},
		{
			Type: "function",
			Function: ollama.ToolFunction{
				Name:        "create_dir",
				Description: "Create a directory and any missing parents.",
				Parameters: ollama.ToolParameters{
					Type: "object",
					Properties: map[string]ollama.ToolPropertySchema{
						"path": {Type: "string", Description: "Directory path relative to the project root"},
					},
					Required: []string{"path"},
				},
			},
		},
		{
			Type: "function",
			Function: ollama.ToolFunction{
				Name:        "list_dir",
				Description: "List all files and directories (including hidden ones that start with '.') inside a directory. Call repeatedly to explore subdirectories. Use \".\" for the project root, \".specs/features/NAME\" to inspect a feature folder, etc.",
				Parameters: ollama.ToolParameters{
					Type: "object",
					Properties: map[string]ollama.ToolPropertySchema{
						"path": {Type: "string", Description: "Directory path relative to the project root. Hidden directories like .specs/ are fully accessible."},
					},
					Required: []string{"path"},
				},
			},
		},
	}
}

// Executor returns a ToolExecutor bound to projectRoot.
func Executor(projectRoot string) ollama.ToolExecutor {
	return func(toolName string, args map[string]any) string {
		result, err := execute(projectRoot, toolName, args)
		if err != nil {
			return "ERROR: " + err.Error()
		}
		return result
	}
}

func execute(projectRoot, toolName string, args map[string]any) (string, error) {
	switch toolName {
	case "write_file":
		return writeFile(projectRoot, args)
	case "read_file":
		return readFile(projectRoot, args)
	case "edit_file":
		return editFile(projectRoot, args)
	case "delete_file":
		return deleteFile(projectRoot, args)
	case "create_dir":
		return createDir(projectRoot, args)
	case "list_dir":
		return listDir(projectRoot, args)
	default:
		return "", fmt.Errorf("unknown tool: %s", toolName)
	}
}

// --- individual tool implementations ---

func isSpecsMutationBlocked(rel string) error {
	clean := filepath.ToSlash(filepath.Clean(rel))
	if clean == ".specs" || strings.HasPrefix(clean, ".specs/") {
		return fmt.Errorf("cannot modify %s — .specs/ is managed by tsll (specify/design/tasks/implement). Task status is updated automatically in .specs/features/<feature-slug>/tasks.md", rel)
	}
	return nil
}

func writeFile(root string, args map[string]any) (string, error) {
	path, content, err := requirePathAndString(root, args, "content")
	if err != nil {
		return "", err
	}
	rel, _ := filepath.Rel(root, path)
	if err := isSpecsMutationBlocked(rel); err != nil {
		return "", err
	}
	if err := validateWriteContent(rel, content); err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("create dirs: %w", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("write: %w", err)
	}
	return fmt.Sprintf("wrote %s (%d bytes)", rel, len(content)), nil
}

func validateWriteContent(relPath, content string) error {
	if strings.TrimSpace(content) == "" {
		return fmt.Errorf("write_file content is empty for %s", relPath)
	}
	if !strings.HasSuffix(relPath, ".go") {
		return nil
	}
	trimmed := strings.TrimSpace(content)
	if !strings.HasPrefix(trimmed, "package ") {
		return fmt.Errorf("Go file %s must start with a package declaration", relPath)
	}
	if strings.Contains(content, "import (") && !strings.Contains(content, ")") {
		return fmt.Errorf("Go file %s looks truncated (unclosed import block)", relPath)
	}
	if strings.Count(content, "{") > strings.Count(content, "}") {
		return fmt.Errorf("Go file %s looks truncated (unclosed braces)", relPath)
	}
	return nil
}

func readFile(root string, args map[string]any) (string, error) {
	path, err := resolvePath(root, args)
	if err != nil {
		return "", err
	}
	if info, statErr := os.Stat(path); statErr == nil && info.IsDir() {
		entries, listErr := os.ReadDir(path)
		if listErr != nil {
			return "", fmt.Errorf("%s is a directory — use list_dir(path=%q) instead (list failed: %v)", path, path, listErr)
		}
		rel, _ := filepath.Rel(root, path)
		if rel == "." {
			rel = "."
		}
		var names []string
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() {
				name += "/"
			}
			names = append(names, name)
		}
		if len(names) == 0 {
			names = []string{"(empty)"}
		}
		return "", fmt.Errorf("%s is a directory — use list_dir(path=%q) instead. Entries: %s",
			rel, rel, strings.Join(names, ", "))
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read: %w", err)
	}
	return string(data), nil
}

func editFile(root string, args map[string]any) (string, error) {
	path, err := resolvePath(root, args)
	if err != nil {
		return "", err
	}
	rel, _ := filepath.Rel(root, path)
	if err := isSpecsMutationBlocked(rel); err != nil {
		return "", err
	}
	oldContent, ok := stringArg(args, "old_content")
	if !ok {
		return "", fmt.Errorf("old_content required")
	}
	newContent, ok := stringArg(args, "new_content")
	if !ok {
		return "", fmt.Errorf("new_content required")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read: %w", err)
	}
	original := string(data)
	count := strings.Count(original, oldContent)
	if count == 0 {
		return "", fmt.Errorf("old_content not found in %s", filepath.Base(path))
	}
	if count > 1 {
		return "", fmt.Errorf("old_content matches %d times in %s — make it more specific", count, filepath.Base(path))
	}
	updated := strings.Replace(original, oldContent, newContent, 1)
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		return "", fmt.Errorf("write: %w", err)
	}
	return fmt.Sprintf("edited %s", rel), nil
}

func deleteFile(root string, args map[string]any) (string, error) {
	path, err := resolvePath(root, args)
	if err != nil {
		return "", err
	}
	rel, _ := filepath.Rel(root, path)
	if err := isSpecsMutationBlocked(rel); err != nil {
		return "", err
	}
	if err := os.Remove(path); err != nil {
		return "", fmt.Errorf("delete: %w", err)
	}
	return fmt.Sprintf("deleted %s", rel), nil
}

func createDir(root string, args map[string]any) (string, error) {
	path, err := resolvePath(root, args)
	if err != nil {
		return "", err
	}
	rel, _ := filepath.Rel(root, path)
	if err := isSpecsMutationBlocked(rel); err != nil {
		return "", err
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		return "", fmt.Errorf("mkdir: %w", err)
	}
	return fmt.Sprintf("created dir %s", rel), nil
}

func listDir(root string, args map[string]any) (string, error) {
	path, err := resolvePath(root, args)
	if err != nil {
		return "", err
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return "", fmt.Errorf("readdir: %w", err)
	}
	var lines []string
	for _, e := range entries {
		if e.IsDir() {
			lines = append(lines, e.Name()+"/")
		} else {
			lines = append(lines, e.Name())
		}
	}
	if len(lines) == 0 {
		return "(empty)", nil
	}
	return strings.Join(lines, "\n"), nil
}

// --- helpers ---

func resolvePath(root string, args map[string]any) (string, error) {
	rel, ok := stringArg(args, "path")
	if !ok || strings.TrimSpace(rel) == "" {
		return "", fmt.Errorf("path required")
	}
	if filepath.IsAbs(rel) {
		return rel, nil
	}
	return filepath.Join(root, rel), nil
}

func requirePathAndString(root string, args map[string]any, key string) (string, string, error) {
	path, err := resolvePath(root, args)
	if err != nil {
		return "", "", err
	}
	val, ok := stringArg(args, key)
	if !ok {
		return "", "", fmt.Errorf("%s required", key)
	}
	return path, val, nil
}

func stringArg(args map[string]any, key string) (string, bool) {
	v, ok := args[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}
