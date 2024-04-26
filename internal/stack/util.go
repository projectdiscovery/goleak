package stack

import (
	"fmt"
	"sort"
	"strings"
)

// PrintGraph recursively prints the graph in a tree-like structure, including orphan nodes.
func PrintGraph(buff *strings.Builder, graph map[int][]int, defs map[int]Entry) {
	visited := make(map[int]bool)
	roots := findRoots(graph)

	// Print each root and its subgraph
	for _, root := range roots {
		printSubGraph(buff, root, graph, "", true, defs, visited)
	}

	// Check for any completely disconnected nodes
	for node := range graph {
		if !visited[node] {
			// Print disconnected nodes as new roots
			printSubGraph(buff, node, graph, "", true, defs, visited)
		}
	}
}

// Helper function to print each subgraph
func printSubGraph(buff *strings.Builder, node int, graph map[int][]int, prefix string, isLast bool, defs map[int]Entry, visited map[int]bool) {
	// Mark the current node as visited
	visited[node] = true

	// Print the current node
	if isLast {
		buff.WriteString(fmt.Sprintf("%s└── %d (%s)\n", prefix, node, defs[node].funcCall))
		prefix += "    "
	} else {
		buff.WriteString(fmt.Sprintf("%s├── %d (%s)\n", prefix, node, defs[node].funcCall))
		prefix += "│   "
	}

	// Recursively print each child
	if children, exists := graph[node]; exists {
		for i, child := range children {
			printSubGraph(buff, child, graph, prefix, i == len(children)-1, defs, visited)
		}
	}
}

// Helper function to find roots of the graph
func findRoots(graph map[int][]int) []int {
	isChild := make(map[int]bool)
	for _, children := range graph {
		for _, child := range children {
			isChild[child] = true
		}
	}

	var roots []int
	for node := range graph {
		if !isChild[node] {
			roots = append(roots, node)
		}
	}
	return roots
}

// BuildGraph constructs the graph from a mapping of item to source, including orphan nodes.
func BuildGraph(mapping map[int]int) map[int][]int {
	graph := make(map[int][]int)
	allNodes := make(map[int]bool)

	// Collect all nodes (both items and sources)
	for item, source := range mapping {
		allNodes[item] = true
		allNodes[source] = true
		graph[source] = append(graph[source], item)
	}

	// Ensure all nodes are in the graph, even if they have no children
	for node := range allNodes {
		if _, exists := graph[node]; !exists {
			graph[node] = []int{} // Initialize with empty slice if no children
		}
	}

	// Optional: Sort children for consistent output
	for _, children := range graph {
		sort.Ints(children)
	}
	return graph
}
