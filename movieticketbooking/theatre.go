package movieticketbooking

type Theatre struct {
	ID string
	Name string
	Location string
	Shows []*Show
}

func NewTheatre(id, name, location string) *Theatre {
	return &Theatre{ID: id, Name: name, Location: location}
}