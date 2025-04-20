package diff

// AttributeChange represents a change between two snapshots of an attribute.
type AttributeChange struct {
	Name string
	Old  interface{}
	New  interface{}
}
