package main

import (
	"fmt"
	"net"
	"time"
)

func handleConn(conn net.Conn, world *World, accounts *AccountManager) {
	session := NewTelnetSession(conn)
	defer session.Close()
	username, isAdmin, err := login(session, accounts)
	if err != nil {
		return
	}

	p, err := world.addPlayer(username, session, isAdmin)
	if err != nil {
		_ = session.WriteString(ansi(style("\r\n"+err.Error()+"\r\n", ansiYellow)))
		return
	}

	go func() {
		for out := range p.Output {
			_ = session.WriteString(out)
		}
	}()

	p.Output <- ansi(style("\r\nWelcome to the tiny Go MUD. Type 'help' for commands.", ansiMagenta, ansiBold))
	enterRoom(world, p, "")

	_ = conn.SetReadDeadline(time.Time{})

	for {
		line, err := session.ReadLine()
		if err != nil {
			break
		}
		line = trim(line)
		if line == "" {
			p.Output <- prompt(p)
			continue
		}
		if !p.Alive {
			break
		}
		if quit := dispatch(world, p, line); quit {
			break
		}
		p.Output <- prompt(p)
	}

	p.Alive = false
	world.broadcastToRoom(p.Room, ansi(fmt.Sprintf("\r\n%s leaves.", highlightName(p.Name))), p)
	world.removePlayer(p.Name)
}

func main() {
	addr := ":4000"
	world, err := NewWorld()
	if err != nil {
		panic(err)
	}
	accounts := NewAccountManager()

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		panic(err)
	}
	fmt.Printf("MUD listening on %s (telnet + ANSI ready)\n", addr)

	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}
		go handleConn(conn, world, accounts)
	}
}
