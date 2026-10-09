package project

import (
	"bytes"
	"path"
	"sort"
	"strings"
)

type FileNode struct {
	Name     string      `json:"name"`
	Path     string      `json:"path"`
	Type     string      `json:"type"` // "file" or "directory"
	Size     int         `json:"size,omitempty"`
	Children []*FileNode `json:"children,omitempty"`
}

type ExploreResponse struct {
	Root       string            `json:"root"`
	TotalFiles int               `json:"total_files"`
	Tree       *FileNode         `json:"tree"`
	Files      map[string]string `json:"files"` // path -> file content text
}

// BuildExploreData constructs the hierarchical directory tree and text files map
func BuildExploreData(files map[string][]byte, serviceName string) *ExploreResponse {
	rootNode := &FileNode{
		Name:     serviceName,
		Path:     "",
		Type:     "directory",
		Children: make([]*FileNode, 0),
	}

	textFiles := make(map[string]string, len(files))

	// Sort paths so tree hierarchy is built cleanly
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	for _, relPath := range paths {
		content := files[relPath]
		if isBinary(content) {
			textFiles[relPath] = "[Binary file]"
		} else {
			textFiles[relPath] = string(content)
		}

		insertIntoTree(rootNode, relPath, len(content))
	}

	sortTree(rootNode)

	return &ExploreResponse{
		Root:       serviceName,
		TotalFiles: len(files),
		Tree:       rootNode,
		Files:      textFiles,
	}
}

func insertIntoTree(root *FileNode, filePath string, size int) {
	segments := strings.Split(filePath, "/")
	current := root

	for i, seg := range segments {
		isLeaf := (i == len(segments)-1)
		subPath := path.Join(segments[:i+1]...)

		var found *FileNode
		for _, child := range current.Children {
			if child.Name == seg {
				found = child
				break
			}
		}

		if found == nil {
			nodeType := "directory"
			nodeSize := 0
			if isLeaf {
				nodeType = "file"
				nodeSize = size
			}

			newNode := &FileNode{
				Name:     seg,
				Path:     subPath,
				Type:     nodeType,
				Size:     nodeSize,
				Children: make([]*FileNode, 0),
			}
			current.Children = append(current.Children, newNode)
			current = newNode
		} else {
			current = found
		}
	}
}

func sortTree(node *FileNode) {
	if len(node.Children) == 0 {
		return
	}

	sort.Slice(node.Children, func(i, j int) bool {
		// Directories first
		if node.Children[i].Type != node.Children[j].Type {
			return node.Children[i].Type == "directory"
		}
		return node.Children[i].Name < node.Children[j].Name
	})

	for _, child := range node.Children {
		sortTree(child)
	}
}

// isBinary preserves the preview placeholder convention for NUL-containing data.
func isBinary(data []byte) bool {
	if len(data) > 1024 {
		data = data[:1024]
	}
	return bytes.ContainsRune(data, 0)
}
