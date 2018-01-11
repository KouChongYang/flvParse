package main

import (
	"os"
	"github.com/KouChongYang/flvParse/flv"
	"fmt"
	"flag"
)

var GConfFile string

func ParseCommandLine() {
	flag.StringVar(&GConfFile, "f", " ", "-c file.flv")
	flag.Parse()
}

func main(){
	ParseCommandLine()
	if len(GConfFile)<=1 {
		fmt.Println("./flvparse -f filename.flv")
		return
	}
	fmt.Println(len(GConfFile))
	r, err := os.Open(GConfFile)
	if err != nil {
		fmt.Println("err:",err)
	}

	fdm:=flv.NewDemuxer(r)
	err=fdm.FlvParse()
	fmt.Println(err)
}
