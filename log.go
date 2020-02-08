package main

import (
	"io/ioutil"
	"log"
	"os"
)

var (
	Debug *DiscardLogger
	//Debug *log.Logger
	Info *log.Logger
	Warning *log.Logger
	Error * log.Logger
)

func init(){
	Debug = NewDiscardLogger(ioutil.Discard,"Debug:",log.Ldate | log.Ltime | log.Lshortfile)
	//Debug = log.New(os.Stdout,"Debug:",log.Ldate | log.Ltime | log.Lshortfile)
	Info = log.New(os.Stdout,"Info:",log.Ldate | log.Ltime | log.Lshortfile)
	Warning = log.New(os.Stdout,"Warning:",log.Ldate | log.Ltime | log.Lshortfile)
	Error = log.New(os.Stdout,"Error:",log.Ldate | log.Ltime | log.Lshortfile)
}
