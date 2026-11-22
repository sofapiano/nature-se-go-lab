package notification

type notificationService interface {
}
type controller struct {
	service notificationService
}

func New(service notificationService) *controller {
	return &controller{service: service}
}
