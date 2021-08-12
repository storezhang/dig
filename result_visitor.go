package dig

type resultVisitor interface {
	Visit(result) resultVisitor
	AnnotateWithField(resultObjectField) resultVisitor
	AnnotateWithPosition(idx int) resultVisitor
}
