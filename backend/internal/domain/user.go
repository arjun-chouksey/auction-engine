package domain

type Role string

const (
	RoleBuyer    Role = "BUYER"
	RoleSupplier Role = "SUPPLIER"
)

type User struct {
	ID           string
	FullName     string
	Email        string
	PasswordHash string
	Role         Role
}
