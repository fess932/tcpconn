package main

import (
	"log"
	"tcpconn/pkg/tcpv2"
	"time"
)

func main() {
	// Start Server
	go func() {
		l, err := tcpv2.Listen("127.0.0.1:8080")
		if err != nil {
			log.Fatalf("Listen error: %v", err)
		}
		defer l.Close()
		log.Printf("Server listening on %s", l.Addr())

		for {
			conn, err := l.Accept()
			if err != nil {
				log.Printf("Accept error: %v", err)
				return
			}
			log.Printf("Accepted connection from %s", conn.RemoteAddr())

			go handleConnection(conn)
		}
	}()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Start Client
	log.Println("Client dialing...")
	conn, err := tcpv2.Dial("127.0.0.1:8080")
	if err != nil {
		log.Fatalf("Dial error: %v", err)
	}
	defer conn.Close()
	log.Println("Client connected!")

	// Send Data
	message := "Hello, TCP over UDP!"
	log.Printf("Client sending: %s", message)
	if _, err := conn.Write([]byte(message)); err != nil {
		log.Fatalf("Write error: %v", err)
	}

	// Wait a bit for response (though we don't send one in this simple demo)
	time.Sleep(1 * time.Second)
	log.Println("Demo finished successfully")
}

func handleConnection(conn *tcpv2.Conn) {
	defer conn.Close()
	buf := make([]byte, 1024)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			log.Printf("Read error: %v", err)
			return
		}
		log.Printf("Server received: %s", string(buf[:n]))
	}
}
