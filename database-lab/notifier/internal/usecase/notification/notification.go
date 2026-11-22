package notification

type (
	sender interface{}

	bindings interface{}
)

type service struct {
	sender   sender
	bindings bindings
}

func New(sender sender, bindings bindings) *service {
	return &service{
		sender:   sender,
		bindings: bindings,
	}
}
