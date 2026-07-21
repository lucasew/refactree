package graphql

// Store is the project-backed data surface for GraphQL resolvers.
// Implemented by pkg/web to avoid an import cycle with generated GraphQL code.
type Store interface {
	RootDir() string
	Filesystem(ref *string) ([]*FsEntry, error)
	Neighborhood(ref string) (*Neighborhood, error)
	ProjectGraph() (*Neighborhood, error)
	Code(ref string) (*CodeDocument, error)
	Doc(ref string) (*Doc, error)
	Node(id string) (*GraphNode, error)
	Nodes(ids []string) ([]*GraphNode, error)
}

// Resolver is the root GraphQL dependency injection root.
type Resolver struct {
	Store Store
}
