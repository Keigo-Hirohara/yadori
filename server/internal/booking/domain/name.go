package domain

import (
	"errors"
	"strings"
	"unicode/utf8"
)

var (
	ErrInvalidName = errors.New("姓名はそれぞれ1〜50文字で入力してください")
)

const maxNameLength = 50

type Name struct {
	firstName string
	lastName  string
}

func NewName(firstName, lastName string) (Name, error) {
	first := strings.TrimSpace(firstName)
	last := strings.TrimSpace(lastName)
	if err := validNamePart(first); err != nil {
		return Name{}, err
	}
	if err := validNamePart(last); err != nil {
		return Name{}, err
	}
	return Name{firstName: first, lastName: last}, nil
}

func (n Name) FirstName() string {
	return n.firstName
}

func (n Name) LastName() string {
	return n.lastName
}

func (n Name) FullName() string {
	return n.lastName + " " + n.firstName
}

func validNamePart(s string) error {
	length := utf8.RuneCountInString(s)
	if length < 1 || length > maxNameLength {
		return ErrInvalidName
	}
	return nil
}
