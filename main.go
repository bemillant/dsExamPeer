package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"strconv"
	"time"

	ping "github.com/bemillant/dsExamPeer/grpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// These are the necessary attributes that each client must have, collected in a peer struct.
type peer struct {
	ping.UnimplementedPingServer
	id            int32
	clients       map[int32]ping.PingClient
	ctx           context.Context
	wantToEnterCS bool
	neighbour     ping.PingClient
	hasToken      bool
}

// Main function.
func main() {
	// Assign ports for the clients. In this implementation, there must be 3.
	// Ports used: 5001, 5002, 5003
	arg1, _ := strconv.ParseInt(os.Args[1], 10, 32)
	ownPort := int32(arg1) + 5001

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	p := &peer{
		id:            ownPort,
		clients:       make(map[int32]ping.PingClient),
		ctx:           ctx,
		neighbour:     nil,
		wantToEnterCS: false,
		hasToken:      false,
	}

	// Set the out for the logs to log.txt. This file is wiped everytime the program is run, but can be analyzed after each.
	setLog()

	// This hardcodes the first client to be in possession of the token.
	if ownPort == 5001 {
		p.hasToken = true

		// This is a place in the code that is guaranteed to only run once, at startup
		// The following method call ensures the critical section is empty in the beginning.
		p.wipeCriticalSection()
	}

	// Create listener tcp on port ownPort
	list, err := net.Listen("tcp", fmt.Sprintf(":%v", ownPort))
	if err != nil {
		log.Fatalf("Failed to listen on port: %v", err)
		fmt.Printf("Failed to listen on port: %v", err)
	} else {
		log.Printf("client with ID: %v now listening on port %v", p.id, ownPort)
		fmt.Printf("you are now listening on port %v", ownPort)
	}

	grpcServer := grpc.NewServer()
	ping.RegisterPingServer(grpcServer, p)

	go func() {
		if err := grpcServer.Serve(list); err != nil {
			log.Fatalf("failed to server %v", err)
			fmt.Printf("failed to server %v", err)
		}
	}()

	// This is where the clients connect to each other. The grpc.WithBlock() call, forces the clients to await each other
	// before the program can run properly.
	for i := 0; i < 3; i++ {
		port := int32(5001) + int32(i)

		if port == ownPort {
			continue
		}

		var conn *grpc.ClientConn
		fmt.Printf("you are trying to dial: %v\n", port)
		log.Printf("client with ID: %v is trying to dial: %v\n", p.id, port)
		insecure := insecure.NewCredentials()
		conn, err := grpc.Dial(fmt.Sprintf(":%v", port), grpc.WithTransportCredentials(insecure), grpc.WithBlock())
		if err != nil {
			log.Fatalf("client with ID: %v could not connect: %s", p.id, err)
			fmt.Printf("you could not connect: %s", err)
		}

		log.Printf("client with ID: %v --- Succesfully dialed to %v\n", p.id, port)
		fmt.Printf("you succesfully dialed to %v\n", port)

		defer conn.Close()
		c := ping.NewPingClient(conn)
		p.clients[port] = c
	}

	// This assigns the clients neighbour, that it will always pass the token to.
	// We do not handle disconnected clients - however it could be implemented with a check, whether a passing of a token succeed within a predetermined time
	// If the token could not be passed to a clients neighbor, the clients just establishes new neighbors.
	p.setNeighbour()

	for {

		//First check if the client has the token and wants to enter CS - then enter the CS
		//(If the client has requested to enter CS whilst not having the token)
		if p.hasToken && p.wantToEnterCS {
			p.handleCriticalSection()
			continue
		} else {
			//n is a random integer (that comes at after a random timeinterval)
			//n is used to emulate a clients decisions - having passing the token being more occuring than requesting to enter and enter CS
			// !! Maybe the handleCS should just be moved out of the switch? !!
			n := giveRandInt()
			switch n % 2 {
			case 0:
				switch n % 3 {
				case 0:
					p.handleCriticalSection()
					continue
				case 1:
					if p.hasToken && p.wantToEnterCS {
						p.handleCriticalSection()
						continue
					}
					p.requestCriticalSection()
					continue
				default:
					continue
				}
			case 1:
				if p.hasToken && p.wantToEnterCS {
					p.handleCriticalSection()
					continue
				} else if p.hasToken {
					p.PassTokenToNeighbour()
				}
				continue
			default:
				continue
			}
		}

		//Implementaion of manual passing and request of token.
		// var message string
		// fmt.Scan(&message)

		// switch message {
		// case "pass":
		// 	p.PassTokenToNeighbour()
		// 	continue
		// case "requestCS":
		// 	p.requestCriticalSection()
		// 	continue
		// case "accessCS":
		// 	p.handleCriticalSection()
		// 	continue
		// default:
		// 	continue
		// }
	}
}

func (p *peer) Token(ctx context.Context, pass *ping.Pass) (*ping.Acknowledgement, error) {
	Ack := &ping.Acknowledgement{
		Message: "Token has succesfully been passed",
	}
	p.hasToken = true

	log.Printf("token has been received from %v", pass.Id)
	return Ack, nil
}

// This function is responsible for passing on the token to the clients neighbour.
func (p *peer) PassTokenToNeighbour() {
	if p.hasToken {
		token := &ping.Pass{
			Message: "Passing on token",
			Id:      p.id,
		}

		ack, err := p.neighbour.Token(p.ctx, token)
		if err != nil {
			fmt.Println("something went wrong when trying to pass the token")
		}

		p.hasToken = false
		log.Printf("Token succesfully passed from client %v to client at port %v with message: %v", p.id, p.getNeighbourID(), ack.Message)
		fmt.Printf("Token succesfully passed to client at port %v with message: %v", p.getNeighbourID(), ack.Message)

	} else {
		log.Printf("client with ID %v tried to pass token, but failed, since it was not in their possession", p.id)
		fmt.Print("you do not possess token, so you cannot pass it to your neighbour")
	}
}

// Gives a random integer to use in switch statement - as well as putting a sleeper on the app
// Only used for emulating the app
func giveRandInt() int32 {
	rand.Seed(time.Now().UnixNano())
	i := rand.Intn(10) // random number between 0 and 10
	time.Sleep(time.Duration(i) * time.Second)

	n := rand.Int31n(5)
	return n
}

// Self-explanatory.
func (p *peer) setNeighbour() {

	if p.id == 5003 {
		p.neighbour = p.clients[5001]
	} else {
		p.neighbour = p.clients[p.id+1]
	}
}

// Self-explanatory.
func (p *peer) getNeighbourID() int32 {

	if p.id == 5003 {
		return 5001
	} else {
		return p.id + 1
	}
}

// This is where the critical section is actually accessed. A string is written to the file critical_section.log
func (p *peer) writeToFile(message string) {

	f, err := os.OpenFile(
		"critical_section.log",
		os.O_APPEND|os.O_CREATE|os.O_WRONLY,
		0644,
	)

	if err != nil {
		log.Println(err)
	}

	defer f.Close()

	if _, err := f.WriteString(message + "\n"); err != nil {
		log.Println(err)
	} else {
		log.Printf("succesfully wrote %v to critical section.", message)
		fmt.Printf("client with ID %v succesfully wrote %v to critical section.", p.id, message)
	}
}

// Self-explanatory. Used for setting up a fresh critical_section.log
func (p *peer) wipeCriticalSection() {
	if err := os.Truncate("critical_section.log", 0); err != nil {
		log.Print("Failed to truncate: %v", err)
		fmt.Print("Failed to truncate: %v", err)
	}
}

// A client requests acccess to the critical section, simply by changing its boolean.
func (p *peer) requestCriticalSection() {
	p.wantToEnterCS = true
	fmt.Printf("requesting to enter the Critical section \n")
	log.Printf("peer with Id: %v now request to enter the Critical section \n", p.id)
}

// Determines whether or not a client will use the oppotunity to access the critical section.
func (p *peer) handleCriticalSection() {

	if p.wantToEnterCS && p.hasToken {
		p.writeToFile(p.generateCSMessage())
		p.wantToEnterCS = false
		log.Printf("client with ID %v no longer wants access to critical section", p.id)
		fmt.Println("you no longer want access to critical section")
	} else if p.hasToken {
		fmt.Println("no request made, so cannot access critical section")
		log.Printf("client with id %v has not made a request for CS, so access cannot be given", p.id)
	} else {
		fmt.Println("you do not have the token, so you cannot access critical section")
		log.Printf("client with ID %v does not have token, so cannot access critical section", p.id)
	}
}

// This allows the user to define what will be written to the critical_section.log file.
func (p *peer) generateCSMessage() string {
	fmt.Println("Input text to write to critical section: ")
	in := bufio.NewReader(os.Stdin)
	message, _ := in.ReadString('\n')
	return message
}

// sets the logger to use a log.txt file instead of the console
func setLog() *os.File {
	// Clears the log.txt file when a new server is started
	if err := os.Truncate("log.txt", 0); err != nil {
		log.Printf("Failed to truncate: %v", err)
	}

	// This connects to the log file/changes the output of the log informaiton to the log.txt file.
	f, err := os.OpenFile("log.txt", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	log.SetOutput(f)
	return f
}
