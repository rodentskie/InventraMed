package root

// Service owns the business logic for the API root endpoint.
type Service interface {
	Greeting() string
}

type service struct{}

func NewService() Service {
	return &service{}
}

func (s *service) Greeting() string {
	return "inventramed REST API"
}
