package system

type user struct {
	uid int
}

func (system) CreateUser(homeDirectory string) (User, error) {
	return nil, nil
}

func (u *user) Remove() error {
	return nil
}
