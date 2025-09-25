package game

import (
	"fmt"
	"strings"
)

const (
	loginBanner = "╔══════════════════════════════════════╗\r\n" +
		"║              LUMENCLAY               ║\r\n" +
		"║  Sculpt your legend in living light  ║\r\n" +
		"╚══════════════════════════════════════╝"
	loginTagline    = "Where imagination takes shape in radiant hues."
	copyrightNotice = "All rights reserved, Copyright 2025 Carl Frank Otto III"
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
	_ = session.WriteString(Ansi("\r\n" + Style(loginBanner, AnsiCyan, AnsiBold) + "\r\n"))
	_ = session.WriteString(Ansi(Style("\r\n"+loginTagline+"\r\n", AnsiGreen)))
	_ = session.WriteString(Ansi(Style("\r\n"+copyrightNotice+"\r\n", AnsiBlue, AnsiDim)))
	_ = session.WriteString(Ansi(Style("\r\nLogin required.\r\n", AnsiMagenta, AnsiBold)))
	for attempts := 0; attempts < 5; attempts++ {
		_ = session.WriteString(Ansi("\r\nUsername: "))
		username, err := session.ReadLine()
		if err != nil {
			return "", false, err
		}
		username = Trim(username)
		if err := validateUsername(username); err != nil {
			_ = session.WriteString(Ansi(Style("\r\n"+err.Error(), AnsiYellow)))
			continue
		}
		if accounts.Exists(username) {
			for tries := 0; tries < 3; tries++ {
				_ = session.WriteString(Ansi("\r\nPassword: "))
				password, err := session.ReadLine()
				if err != nil {
					return "", false, err
				}
				password = Trim(password)
				if accounts.Authenticate(username, password) {
					_ = session.WriteString(Ansi(Style("\r\nWelcome back, "+username+"!", AnsiGreen)))
					return username, accounts.IsAdmin(username), nil
				}
				_ = session.WriteString(Ansi(Style("\r\nIncorrect password.", AnsiYellow)))
			}
			_ = session.WriteString(Ansi("\r\nToo many failed attempts.\r\n"))
			return "", false, fmt.Errorf("authentication failed")
		}

		for {
			_ = session.WriteString(Ansi("\r\nSet a password: "))
			password, err := session.ReadLine()
			if err != nil {
				return "", false, err
			}
			password = Trim(password)
			if err := validatePassword(password); err != nil {
				_ = session.WriteString(Ansi(Style("\r\n"+err.Error(), AnsiYellow)))
				continue
			}
			if err := accounts.Register(username, password); err != nil {
				_ = session.WriteString(Ansi(Style("\r\n"+err.Error(), AnsiYellow)))
				break
			}
			_ = session.WriteString(Ansi(Style("\r\nAccount created. Welcome, "+username+"!", AnsiGreen)))
			return username, accounts.IsAdmin(username), nil
		}
	}
	_ = session.WriteString(Ansi("\r\nLogin cancelled.\r\n"))
	return "", false, fmt.Errorf("login cancelled")
}
