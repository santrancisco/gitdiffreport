package main

import (
	"fmt"
	"log"
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

//Send slackreport to slack
func sendtoslack(slackreport string) {
	debug(slackreport)
	if (slackreport != "") && (!*debugflag) {
		slackreport = "POTENTIAL CREDENTIALS LEAK:\n\n" + slackreport
		notify := slackalert.SlackStruct{URL: slackurl, Uploadtoken: slacktoken, Icon: POLICE, Channel: *slackchannel}
		notify.Sendmsg("Incoming falsch positiv aufmerksam!")
		notify.UploadFile(time.Now().Format("2006-02-01")+".txt", slackreport)
	}
}
