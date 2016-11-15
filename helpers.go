package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"github.com/santrancisco/checkcommit/slackalert"
)

func check(e error) {
	if e != nil {
		//panic(e)
		fmt.Println(e)
		os.Exit(1)
	}
}

func debug(s string) {
	if *debugflag {
		log.Println(s)
	}
}

