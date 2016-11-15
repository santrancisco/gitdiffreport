package main

import (
	"fmt"
	"log"
	"os"

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

