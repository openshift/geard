package geard

import (
	"code.google.com/p/go.crypto/ssh"
	"code.google.com/p/go.crypto/ssh/terminal"
	"errors"
	"fmt"
	"log"
)

type SshServer struct {
}

func (srv *SshServer) ListenAndServe(addr string, config *ssh.ServerConfig) error {
	if addr == "" {
		addr = ":2022"
	}
	l, e := ssh.Listen("tcp", addr, config)
	if e != nil {
		return e
	}
	return srv.Serve(l)
}

func (srv *SshServer) Serve(listener *ssh.Listener) error {
	defer listener.Close()

	for {
		log.Print("Waiting for incoming SSH connection")
		conn, err := listener.Accept()
		log.Print("Found an incoming SSH connection")
		if err != nil {
			return errors.New("ssh: failed to accept incoming connection: " + err.Error())
		}
		go func() {
			if err := conn.Handshake(); err != nil {
				log.Printf("ssh: failed to handshake:", err)
				return
			}

			// A ServerConn multiplexes several channels, which must
			// themselves be Accepted.
			for {
				// Accept reads from the connection, demultiplexes packets
				// to their corresponding channels and returns when a new
				// channel request is seen. Some goroutine must always be
				// calling Accept; otherwise no messages will be forwarded
				// to the channels.
				channel, err := conn.Accept()
				if err != nil {
					log.Printf("ssh: failed to accept channel:", err)
					return
				}

				// Channels have a type, depending on the application level
				// protocol intended. In the case of a shell, the type is
				// "session" and ServerShell may be used to present a simple
				// terminal interface.
				if channel.ChannelType() != "session" {
					channel.Reject(ssh.UnknownChannelType, "unknown channel type")
					continue
				}
				channel.Accept()

				term := terminal.NewTerminal(channel, "> ")
				serverTerm := &ssh.ServerTerminal{
					Term:    term,
					Channel: channel,
				}
				go func() {
					defer channel.Close()
					for {
						line, err := serverTerm.ReadLine()
						if err != nil {
							break
						}
						fmt.Println(line)
					}
				}()
			}
		}()
	}
}
