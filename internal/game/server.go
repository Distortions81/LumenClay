package game

import (
	"fmt"
	"net"
	"time"
)

type Dispatcher func(world *World, player *Player, line string) bool

func handleConn(conn net.Conn, world *World, accounts *AccountManager, dispatcher Dispatcher) {
	session := NewTelnetSession(conn)
	defer session.Close()
	username, isAdmin, err := login(session, accounts)
	if err != nil {
		return
	}

	p, err := world.addPlayer(username, session, isAdmin)
	if err != nil {
		_ = session.WriteString(Ansi(Style("\r\n"+err.Error()+"\r\n", AnsiYellow)))
		return
	}

	go func() {
		for out := range p.Output {
			_ = session.WriteString(out)
		}
	}()

	p.Output <- Ansi(Style("\r\nWelcome to the tiny Go MUD. Type 'help' for commands.", AnsiMagenta, AnsiBold))
	EnterRoom(world, p, "")

	_ = conn.SetReadDeadline(time.Time{})

	for {
		line, err := session.ReadLine()
		if err != nil {
			break
		}
		line = Trim(line)
		if line == "" {
			p.Output <- Prompt(p)
			continue
		}
		if !p.Alive {
			break
		}
		if quit := dispatcher(world, p, line); quit {
			break
		}
		p.Output <- Prompt(p)
	}

	p.Alive = false
	world.BroadcastToRoom(p.Room, Ansi(fmt.Sprintf("\r\n%s leaves.", HighlightName(p.Name))), p)
	world.removePlayer(p.Name)
}

func ListenAndServe(addr, accountsPath string, dispatcher Dispatcher) error {
	world, err := NewWorld()
	if err != nil {
		return err
	}
	accounts, err := NewAccountManager(accountsPath)
	if err != nil {
		return err
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	fmt.Printf("MUD listening on %s (telnet + ANSI ready)\n", ln.Addr())

	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}
		go handleConn(conn, world, accounts, dispatcher)
	}

	// Unreachable, but included for completeness.
	// nolint:nilerr
	return nil
}
