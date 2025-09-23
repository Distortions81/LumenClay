package main

import (
	"fmt"
	"strings"
)

func validateUsername(name string) error {
	if name == "" {
		return fmt.Errorf("name cannot be empty")
	}
	if strings.ContainsAny(name, " \t\r\n") {
		return fmt.Errorf("name cannot contain spaces")
	}
	if len(name) > 24 {
		return fmt.Errorf("name must be 24 characters or fewer")
	}
	return nil
}

func validatePassword(password string) error {
	if password == "" {
		return fmt.Errorf("password cannot be blank")
	}
	if len(password) < 6 {
		return fmt.Errorf("password must be at least 6 characters")
	}
	return nil
}

func login(session *TelnetSession, accounts *AccountManager) (string, bool, error) {
	_ = session.WriteString(ansi(style("\r\nLogin required.\r\n", ansiMagenta, ansiBold)))
	for attempts := 0; attempts < 5; attempts++ {
		_ = session.WriteString(ansi("\r\nUsername: "))
		username, err := session.ReadLine()
		if err != nil {
			return "", false, err
		}
		username = trim(username)
		if err := validateUsername(username); err != nil {
			_ = session.WriteString(ansi(style("\r\n"+err.Error(), ansiYellow)))
			continue
		}
		if accounts.Exists(username) {
			for tries := 0; tries < 3; tries++ {
				_ = session.WriteString(ansi("\r\nPassword: "))
				password, err := session.ReadLine()
				if err != nil {
					return "", false, err
				}
				password = trim(password)
				if accounts.Authenticate(username, password) {
					_ = session.WriteString(ansi(style("\r\nWelcome back, "+username+"!", ansiGreen)))
					return username, strings.EqualFold(username, "admin"), nil
				}
				_ = session.WriteString(ansi(style("\r\nIncorrect password.", ansiYellow)))
			}
			_ = session.WriteString(ansi("\r\nToo many failed attempts.\r\n"))
			return "", false, fmt.Errorf("authentication failed")
		}

		for {
			_ = session.WriteString(ansi("\r\nSet a password: "))
			password, err := session.ReadLine()
			if err != nil {
				return "", false, err
			}
			password = trim(password)
			if err := validatePassword(password); err != nil {
				_ = session.WriteString(ansi(style("\r\n"+err.Error(), ansiYellow)))
				continue
			}
			if err := accounts.Register(username, password); err != nil {
				_ = session.WriteString(ansi(style("\r\n"+err.Error(), ansiYellow)))
				break
			}
			_ = session.WriteString(ansi(style("\r\nAccount created. Welcome, "+username+"!", ansiGreen)))
			return username, strings.EqualFold(username, "admin"), nil
		}
	}
	_ = session.WriteString(ansi("\r\nLogin cancelled.\r\n"))
	return "", false, fmt.Errorf("login cancelled")
}
