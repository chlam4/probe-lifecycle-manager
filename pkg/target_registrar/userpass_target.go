package target_registrar

import "gopkg.in/yaml.v2"

// UserPassTarget describes a structure for targets using username and password; this defines our test targets here
type UserPassTarget struct {
	Id        string
	Probetype string
	Username  string
	Password  string
}

func (t UserPassTarget) GetId() string {
	return t.Id
}

func (t UserPassTarget) GetProbeType() string {
	return t.Probetype
}

func (t UserPassTarget) Bytes() ([]byte, error) {
	return yaml.Marshal(&t)
}

// Make sure UserPassTarget implements the Target interface, or a compilation error will result
var _ Target = (*UserPassTarget)(nil)

// Unmarshal the input byte array into a UserPassTarget object
func UserPassTargetFromBytes(bytes []byte, target *UserPassTarget) (error) {
	return yaml.Unmarshal(bytes, &target)
}