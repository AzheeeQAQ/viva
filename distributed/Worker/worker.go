package main

import (
	"fmt"
	"net"
	"net/rpc"
)

func main() {
	lnWork, _ := net.Listen("tcp", ":8010")
	fmt.Println("work listening")
	rpc.Accept(lnWork)
	defer lnWork.Close()
}
