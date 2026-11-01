package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/CoderYkj/HapeGateway/gateway"
)

func main() {
	secret := flag.String("secret", "", "HMAC secret")
	world := flag.Uint("world", 1, "world id")
	player := flag.Uint64("player", 1, "player id")
	ttl := flag.Duration("ttl", time.Hour, "token ttl")
	flag.Parse()
	if *secret == "" {
		log.Fatal("secret required")
	}
	fmt.Println(gateway.IssueToken([]byte(*secret), uint32(*world), *player, *ttl))
}
