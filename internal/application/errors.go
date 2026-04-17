package application

import "fmt"

type Failure struct {
	Code    string
	Message string
	State   any
	NextMode string
	Next    []any
}

func (f *Failure) Error() string {
	return f.Message
}

type ErrCharterExists struct {
	Charter string
}

func (e ErrCharterExists) Error() string {
	return fmt.Sprintf("charter %q already exists", e.Charter)
}
